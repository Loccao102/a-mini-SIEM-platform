package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/ingest"
)

func main() {
	redisURL := flag.String("redis-url", env("REDIS_URL", "redis://localhost:6379/0"), "Redis URL")
	stream := flag.String("stream", env("REDIS_STREAM", ingest.DefaultStream), "Redis Stream name")
	sourceType := flag.String("source-type", env("SOURCE_TYPE", "generic"), "Source type attached to each log")
	hostname := flag.String("hostname", env("HOSTNAME", "unknown"), "Origin hostname")
	agentID := flag.String("agent-id", env("AGENT_ID", "manual-ingest"), "Origin agent ID")
	file := flag.String("file", "", "Read newline-delimited logs from a file; stdin when empty")
	flag.Parse()

	client, err := ingest.NewClient(*redisURL, *stream)
	if err != nil {
		log.Fatalf("create Redis client: %v", err)
	}
	defer client.Close()
	if err := client.Ping(context.Background()); err != nil {
		log.Fatalf("connect to Redis: %v", err)
	}

	reader := io.Reader(os.Stdin)
	if *file != "" {
		input, err := os.Open(*file)
		if err != nil {
			log.Fatalf("open log file: %v", err)
		}
		defer input.Close()
		reader = input
	}

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		if scanner.Text() == "" {
			continue
		}
		id, err := client.Publish(context.Background(), ingest.Message{
			Raw:        scanner.Text(),
			SourceType: *sourceType,
			Hostname:   *hostname,
			AgentID:    *agentID,
			ReceivedAt: time.Now(),
		})
		if err != nil {
			log.Fatalf("publish log: %v", err)
		}
		fmt.Println(id)
	}
	if err := scanner.Err(); err != nil {
		log.Fatalf("read logs: %v", err)
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
