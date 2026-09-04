package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/dedup"
	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/dlq"
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
	redisOpts, err := redis.ParseURL(env("REDIS_URL", "redis://localhost:6379/0"))
	if err != nil {
		log.Fatalf("parse redis url: %v", err)
	}
	redisClient := redis.NewClient(redisOpts)
	defer redisClient.Close()
	dlqManager := dlq.New(redisClient)
	if err := dlqManager.EnsureStreams(context.Background()); err != nil {
		log.Fatalf("ensure dlq streams: %v", err)
	}
	elastic := storage.NewElasticsearch(env("ELASTICSEARCH_URL", "http://localhost:9200"))
	dedupManager := dedup.NewManager(redisClient)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := elastic.EnsureIndex(ctx); err != nil {
		log.Fatalf("prepare Elasticsearch index: %v", err)
	}
	log.Printf("parser consuming Redis Stream %s", env("REDIS_STREAM", ingest.DefaultStream))
	processEvents := func(ctx context.Context, events []parser.NormalizedEvent) error {
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
	if err := consumer.ConsumeBatchWithFailureHandler(ctx, envInt("PARSER_BATCH_SIZE", 100), envInt("PARSER_WORKERS", 4), processEvents, func(ctx context.Context, message redis.XMessage, processingErr error) error {
		payload, err := json.Marshal(message.Values)
		if err != nil {
			return fmt.Errorf("encode failed event %s: %w", message.ID, err)
		}
		source := value(message.Values, "source_type")
		if source == "" {
			source = "parser"
		}
		_, err = dlqManager.SendToRetry(ctx, message.ID, string(payload), 0, source, map[string]string{"error": processingErr.Error()})
		return err
	}); err != nil && ctx.Err() == nil {
		log.Fatal(err)
	}
}

func value(values map[string]any, key string) string {
	if raw, ok := values[key]; ok {
		return fmt.Sprint(raw)
	}
	return ""
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
