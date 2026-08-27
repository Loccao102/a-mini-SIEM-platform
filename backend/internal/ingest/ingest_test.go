package ingest

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestPublishWritesRawLogFields(t *testing.T) {
	server := miniredis.RunT(t)
	client := &Client{redis: redis.NewClient(&redis.Options{Addr: server.Addr()}), stream: DefaultStream}
	defer client.Close()
	receivedAt := time.Date(2026, time.August, 27, 12, 0, 0, 0, time.UTC)

	_, err := client.Publish(context.Background(), Message{
		Raw:        "Failed password for root from 192.0.2.10",
		SourceType: "linux_sshd",
		Hostname:   "web-01",
		AgentID:    "agent-01",
		ReceivedAt: receivedAt,
	})
	if err != nil {
		t.Fatalf("publish raw log: %v", err)
	}

	entries, err := client.redis.XRange(context.Background(), DefaultStream, "-", "+").Result()
	if err != nil {
		t.Fatalf("read stream: %v", err)
	}
	if len(entries) != 1 || entries[0].Values["raw"] != "Failed password for root from 192.0.2.10" {
		t.Fatalf("unexpected stream entry: %#v", entries)
	}
	if entries[0].Values["source_type"] != "linux_sshd" || entries[0].Values["received_at"] != receivedAt.Format(time.RFC3339Nano) {
		t.Fatalf("unexpected metadata: %#v", entries[0].Values)
	}
}
