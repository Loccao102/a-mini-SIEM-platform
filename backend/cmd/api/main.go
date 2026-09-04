package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/api"
	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/apikey"
	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/auth"
	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/dedup"
	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/dlq"
	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/health"
	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/ingest"
	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/metrics"
	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/parser"
	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/ratelimit"
	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/ruleengine"
	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/storage"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	postgres, err := storage.OpenPostgres(ctx, env("POSTGRES_URL", "postgres://siem:siem_dev_password@localhost:5432/siem?sslmode=disable"))
	if err != nil {
		log.Fatalf("connect PostgreSQL: %v", err)
	}
	defer postgres.Close()
	authManager := auth.NewManager(env("JWT_SECRET", "change-me-in-production"))
	if err := ensureSeedData(ctx, postgres, authManager, env("ADMIN_EMAIL", "admin@example.com"), env("ADMIN_PASSWORD", "admin"), env("MODE", "production")); err != nil {
		log.Fatalf("ensure seed data: %v", err)
	}
	ingestClient, err := ingest.NewClient(env("REDIS_URL", "redis://localhost:6379/0"), env("REDIS_STREAM", ingest.DefaultStream))
	if err != nil {
		log.Fatalf("create ingest client: %v", err)
	}
	defer ingestClient.Close()
	// Create Redis client for dedup manager
	redisOpts, err := redis.ParseURL(env("REDIS_URL", "redis://localhost:6379/0"))
	if err != nil {
		log.Fatalf("parse redis url: %v", err)
	}
	redisClient := redis.NewClient(redisOpts)
	dedupManager := dedup.NewManager(redisClient)

	// Initialize API key manager for agent authentication
	apiKeyMgr := apikey.New(postgres)

	// Initialize rate limiter (1000 requests per minute per hostname)
	rateLimiter := ratelimit.New(redisClient, 1000, 1*time.Minute)

	// Initialize DLQ manager for failed event handling
	dlqManager := dlq.New(redisClient)
	if err := dlqManager.EnsureStreams(ctx); err != nil {
		log.Fatalf("ensure dlq streams: %v", err)
	}

	// Initialize health checker
	healthChecker := health.New(postgres, redisClient)

	// Initialize metrics collection
	queueMetrics := metrics.New()

	// Create handler with all components
	handler := api.New(postgres, storage.NewElasticsearch(env("ELASTICSEARCH_URL", "http://localhost:9200")), ingestClient, authManager, dedupManager, apiKeyMgr, rateLimiter, dlqManager, healthChecker, queueMetrics)
	elastic := storage.NewElasticsearch(env("ELASTICSEARCH_URL", "http://localhost:9200"))
	if err := elastic.EnsureIndex(ctx); err != nil {
		log.Fatalf("prepare Elasticsearch index: %v", err)
	}
	consumer, err := parser.NewConsumer(env("REDIS_URL", "redis://localhost:6379/0"), env("REDIS_STREAM", ingest.DefaultStream), env("PARSER_GROUP", parser.DefaultConsumerGroup), env("PARSER_CONSUMER", hostname()))
	if err != nil {
		log.Fatalf("create parser consumer: %v", err)
	}
	defer consumer.Close()
	engine, err := ruleengine.New(postgres, env("REDIS_URL", "redis://localhost:6379/0"), env("TELEGRAM_BOT_TOKEN", ""), env("TELEGRAM_CHAT_ID", ""))
	if err != nil {
		log.Fatalf("create rule engine: %v", err)
	}
	defer engine.Close()
	processEvents := func(ctx context.Context, events []parser.NormalizedEvent) error {
		bulk := make([]any, len(events))
		for index := range events {
			bulk[index] = events[index]
		}
		if err := elastic.BulkIndexEvents(ctx, bulk); err != nil {
			return err
		}
		for _, event := range events {
			if err := engine.Process(ctx, event); err != nil {
				return err
			}
		}
		return nil
	}
	go func() {
		err := dlqManager.ConsumeRetry(ctx, env("PARSER_RETRY_CONSUMER", hostname()+"-retry"), int64(envInt("PARSER_BATCH_SIZE", 100)), func(ctx context.Context, message *dlq.DeadLetterMessage) error {
			var values map[string]any
			if err := json.Unmarshal(message.Payload, &values); err != nil {
				return fmt.Errorf("decode retry payload: %w", err)
			}
			return processEvents(ctx, []parser.NormalizedEvent{parser.Parse(redis.XMessage{ID: message.OriginalID, Values: values})})
		})
		if err != nil && ctx.Err() == nil {
			log.Printf("retry consumer stopped: %v", err)
		}
	}()
	go func() {
		err := consumer.ConsumeBatchWithFailureHandler(ctx, envInt("PARSER_BATCH_SIZE", 100), envInt("PARSER_WORKERS", 4), processEvents, func(ctx context.Context, message redis.XMessage, processingErr error) error {
			payload, err := json.Marshal(message.Values)
			if err != nil {
				return fmt.Errorf("encode failed event %s: %w", message.ID, err)
			}
			_, err = dlqManager.SendToRetry(ctx, message.ID, string(payload), 0, "parser", map[string]string{"error": processingErr.Error()})
			return err
		})
		if err != nil && ctx.Err() == nil {
			log.Printf("parser stopped: %v", err)
		}
	}()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.Handle("/api/", handler.Routes())
	server := &http.Server{Addr: env("API_ADDR", ":8080"), Handler: mux}

	go func() {
		log.Printf("SIEM API listening on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
}

func ensureSeedData(ctx context.Context, postgres *pgxpool.Pool, manager *auth.Manager, defaultAdminEmail, defaultAdminPass, mode string) error {
	// Apply columns migration for alerts table
	_, _ = postgres.Exec(ctx, `
		ALTER TABLE alerts ADD COLUMN IF NOT EXISTS entity_key TEXT;
		ALTER TABLE alerts ADD COLUMN IF NOT EXISTS occurrences INT NOT NULL DEFAULT 1;
		ALTER TABLE alerts ADD COLUMN IF NOT EXISTS last_seen TIMESTAMPTZ NOT NULL DEFAULT now();
		CREATE INDEX IF NOT EXISTS idx_alerts_rule_entity ON alerts (rule_id, entity_key, status);
	`)

	users := []struct {
		Email, Password, Name, Role string
	}{
		{Email: defaultAdminEmail, Password: defaultAdminPass, Name: "System Administrator", Role: "admin"},
	}
	if mode == "develop" {
		users = append(users,
			struct{ Email, Password, Name, Role string }{Email: "admin@example.com", Password: "admin", Name: "SOC Administrator", Role: "admin"},
			struct{ Email, Password, Name, Role string }{Email: "analyst@example.com", Password: "analyst", Name: "SOC Analyst", Role: "analyst"},
			struct{ Email, Password, Name, Role string }{Email: "viewer@example.com", Password: "viewer", Name: "SOC Observer", Role: "viewer"},
		)
	}

	for _, u := range users {
		var exists bool
		if err := postgres.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE lower(email)=lower($1))`, u.Email).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			hash, err := auth.HashPassword(u.Password)
			if err != nil {
				return err
			}
			_, err = postgres.Exec(ctx, `INSERT INTO users (email, password_hash, display_name, role) VALUES ($1, $2, $3, $4)`, u.Email, hash, u.Name, u.Role)
			if err != nil {
				return err
			}
		}
	}

	if mode != "develop" {
		return nil
	}

	// Seed demo assets only for local development.
	var assetCount int
	if err := postgres.QueryRow(ctx, `SELECT COUNT(*) FROM assets`).Scan(&assetCount); err != nil {
		return err
	}
	if assetCount == 0 {
		demoAssets := []struct {
			Hostname, IP, OS, Criticality, Owner string
		}{
			{"web-prod-01", "192.168.1.10", "linux", "critical", "SRE Team"},
			{"db-prod-01", "192.168.1.20", "linux", "critical", "DBA Team"},
			{"win-dc-01", "192.168.1.30", "windows", "critical", "IT Admin"},
			{"web-staging-01", "192.168.1.40", "linux", "high", "Dev Team"},
			{"desktop-dev-01", "192.168.1.50", "windows", "medium", "Dev Team"},
			{"monitoring-01", "192.168.1.60", "linux", "high", "SRE Team"},
			{"DESKTOP-OVVPOR2", "192.168.1.100", "windows", "medium", ""},
		}
		for _, a := range demoAssets {
			owner := interface{}(a.Owner)
			if a.Owner == "" {
				owner = nil
			}
			_, err := postgres.Exec(ctx, `INSERT INTO assets (hostname, ip_address, os_type, criticality, owner) VALUES ($1, $2::inet, $3, $4, $5)`, a.Hostname, a.IP, a.OS, a.Criticality, owner)
			if err != nil {
				log.Printf("seed asset %s: %v", a.Hostname, err)
			}
		}
	}
	return nil
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func hostname() string {
	value, err := os.Hostname()
	if err != nil {
		return "api-1"
	}
	return value
}

func envInt(key string, fallback int) int {
	value, err := strconv.Atoi(env(key, strconv.Itoa(fallback)))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
