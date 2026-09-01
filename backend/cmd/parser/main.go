package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/dedup"
	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/ingest"
	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/parser"
	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/storage"
	"github.com/redis/go-redis/v9"
)

func main() {
	consumer, err := parser.NewConsumer(env("REDIS_URL", "redis://localhost:6379/0"), env("REDIS_STREAM", ingest.DefaultStream), env("PARSER_GROUP", parser.DefaultConsumerGroup), env("PARSER_CONSUMER", hostname()))
	if err != nil {
		log.Fatalf("create parser consumer: %v", err)
	}
	defer consumer.Close()
	elastic := storage.NewElasticsearch(env("ELASTICSEARCH_URL", "http://localhost:9200"))
	// Create Redis client for dedup manager
	redisOpts, err := redis.ParseURL(env("REDIS_URL", "redis://localhost:6379/0"))
	if err != nil {
		log.Fatalf("parse redis url: %v", err)
	}
	redisClient := redis.NewClient(redisOpts)
	dedupManager := dedup.NewManager(redisClient)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := elastic.EnsureIndex(ctx); err != nil {
		log.Fatalf("prepare Elasticsearch index: %v", err)
	}
	log.Printf("parser consuming Redis Stream %s", env("REDIS_STREAM", ingest.DefaultStream))
	if err := consumer.ConsumeBatch(ctx, envInt("PARSER_BATCH_SIZE", 100), envInt("PARSER_WORKERS", 4), func(ctx context.Context, events []parser.NormalizedEvent) error {
		bulk := make([]any, len(events))
		for index, event := range events {
			// Track event vào dedup Redis
			if group, err := dedupManager.TrackEvent(ctx, event.Fingerprint, event.EventID, event.EventTime); err == nil && group != nil {
				event.DuplicateCount = group.Count
				event.FirstSeen = group.FirstSeen
				event.LastSeen = group.LastSeen
			} else if err != nil {
				log.Printf("dedup track error: %v", err)
			}
			encoded, err := json.Marshal(event)
			if err != nil {
				return err
			}
			log.Printf("normalized event: %s", encoded)
			bulk[index] = event
		}
		return elastic.BulkIndexEvents(ctx, bulk)
	}); err != nil && ctx.Err() == nil {
		log.Fatal(err)
	}
}

func hostname() string {
	value, err := os.Hostname()
	if err != nil {
		return "parser-1"
	}
	return value
}
func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	value, err := strconv.Atoi(env(key, strconv.Itoa(fallback)))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
