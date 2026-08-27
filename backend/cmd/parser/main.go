package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/ingest"
	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/parser"
	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/storage"
)

func main() {
	consumer, err := parser.NewConsumer(env("REDIS_URL", "redis://localhost:6379/0"), env("REDIS_STREAM", ingest.DefaultStream), env("PARSER_GROUP", parser.DefaultConsumerGroup), env("PARSER_CONSUMER", hostname()))
	if err != nil {
		log.Fatalf("create parser consumer: %v", err)
	}
	defer consumer.Close()
	elastic := storage.NewElasticsearch(env("ELASTICSEARCH_URL", "http://localhost:9200"))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := elastic.EnsureIndex(ctx); err != nil {
		log.Fatalf("prepare Elasticsearch index: %v", err)
	}
	log.Printf("parser consuming Redis Stream %s", env("REDIS_STREAM", ingest.DefaultStream))
	if err := consumer.Consume(ctx, func(ctx context.Context, event parser.NormalizedEvent) error {
		encoded, err := json.Marshal(event)
		if err == nil {
			log.Printf("normalized event: %s", encoded)
		}
		if err != nil { return err }
		return elastic.IndexEvent(ctx, event)
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
