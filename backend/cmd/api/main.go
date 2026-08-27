package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/api"
	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/auth"
	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/ingest"
	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/parser"
	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/ruleengine"
	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/storage"
	"github.com/jackc/pgx/v5/pgxpool"
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
	if err := ensureAdmin(ctx, postgres, authManager, env("ADMIN_EMAIL", "admin@example.com"), env("ADMIN_PASSWORD", "change-me-now")); err != nil {
		log.Fatalf("ensure admin user: %v", err)
	}
	ingestClient, err := ingest.NewClient(env("REDIS_URL", "redis://localhost:6379/0"), env("REDIS_STREAM", ingest.DefaultStream))
	if err != nil {
		log.Fatalf("create ingest client: %v", err)
	}
	defer ingestClient.Close()
	handler := api.New(postgres, storage.NewElasticsearch(env("ELASTICSEARCH_URL", "http://localhost:9200")), ingestClient, authManager)
	elastic := storage.NewElasticsearch(env("ELASTICSEARCH_URL", "http://localhost:9200"))
	if err := elastic.EnsureIndex(ctx); err != nil {
		log.Fatalf("prepare Elasticsearch index: %v", err)
	}
	consumer, err := parser.NewConsumer(env("REDIS_URL", "redis://localhost:6379/0"), env("REDIS_STREAM", ingest.DefaultStream), env("PARSER_GROUP", parser.DefaultConsumerGroup), env("PARSER_CONSUMER", "api-parser"))
	if err != nil {
		log.Fatalf("create parser consumer: %v", err)
	}
	defer consumer.Close()
	engine, err := ruleengine.New(postgres, env("REDIS_URL", "redis://localhost:6379/0"), env("TELEGRAM_BOT_TOKEN", ""), env("TELEGRAM_CHAT_ID", ""))
	if err != nil {
		log.Fatalf("create rule engine: %v", err)
	}
	defer engine.Close()
	go func() {
		err := consumer.Consume(ctx, func(ctx context.Context, event parser.NormalizedEvent) error {
			if err := elastic.IndexEvent(ctx, event); err != nil {
				return err
			}
			return engine.Process(ctx, event)
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

func ensureAdmin(ctx context.Context, postgres *pgxpool.Pool, manager *auth.Manager, email, password string) error {
	var exists bool
	if err := postgres.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE lower(email)=lower($1))`, email).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	_, err = postgres.Exec(ctx, `INSERT INTO users (email,password_hash,display_name,role) VALUES ($1,$2,$3,'admin')`, email, hash, "System administrator")
	return err
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
