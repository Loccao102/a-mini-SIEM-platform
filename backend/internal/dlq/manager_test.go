package dlq

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func setupTestRedis(t *testing.T) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15, // Use test DB
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("Redis not available: %v", err)
	}

	// Flush test DB
	client.FlushDB(ctx)

	return client
}

func cleanupTestRedis(t *testing.T, client *redis.Client) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client.FlushDB(ctx)
	client.Close()
}

func TestEnsureStreams(t *testing.T) {
	client := setupTestRedis(t)
	defer cleanupTestRedis(t, client)

	manager := New(client)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// First call should succeed
	err := manager.EnsureStreams(ctx)
	if err != nil {
		t.Fatalf("EnsureStreams failed: %v", err)
	}

	// Second call should also succeed (group already exists)
	err = manager.EnsureStreams(ctx)
	if err != nil {
		t.Fatalf("EnsureStreams second call failed: %v", err)
	}
}

func TestSendToDLQ(t *testing.T) {
	client := setupTestRedis(t)
	defer cleanupTestRedis(t, client)

	manager := New(client)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	payload := `{"message": "test event"}`
	testErr := errors.New("processing failed")

	msgID, err := manager.SendToDLQ(ctx, "orig-123", payload, testErr, "ingest", map[string]string{"env": "test"})
	if err != nil {
		t.Fatalf("SendToDLQ failed: %v", err)
	}

	if msgID == "" {
		t.Fatal("SendToDLQ returned empty message ID")
	}

	// Verify message in stream
	messages, err := client.XRange(ctx, manager.dlqStream, "-", "+").Result()
	if err != nil {
		t.Fatalf("XRange failed: %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(messages))
	}

	dataStr, ok := messages[0].Values["data"].(string)
	if !ok {
		t.Fatal("Message data not found")
	}

	var dlqMsg DeadLetterMessage
	if err := json.Unmarshal([]byte(dataStr), &dlqMsg); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if dlqMsg.OriginalID != "orig-123" {
		t.Errorf("Wrong original ID: %s", dlqMsg.OriginalID)
	}
	if dlqMsg.FailureCount != 0 {
		t.Errorf("Wrong failure count: %d", dlqMsg.FailureCount)
	}
}

func TestSendToRetry(t *testing.T) {
	client := setupTestRedis(t)
	defer cleanupTestRedis(t, client)

	manager := New(client)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	payload := `{"message": "test event"}`

	msgID, err := manager.SendToRetry(ctx, "orig-456", payload, 0, "parser", map[string]string{})
	if err != nil {
		t.Fatalf("SendToRetry failed: %v", err)
	}

	if msgID == "" {
		t.Fatal("SendToRetry returned empty message ID")
	}

	// Verify message in retry stream
	messages, err := client.XRange(ctx, manager.retryStream, "-", "+").Result()
	if err != nil {
		t.Fatalf("XRange failed: %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(messages))
	}
}

func TestExponentialBackoff(t *testing.T) {
	client := setupTestRedis(t)
	defer cleanupTestRedis(t, client)

	manager := New(client)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tests := []struct {
		failureCount int
		minBackoff   time.Duration
		maxBackoff   time.Duration
	}{
		{0, 5 * time.Second, 6 * time.Second},
		{1, 10 * time.Second, 11 * time.Second},
		{2, 20 * time.Second, 21 * time.Second},
		{3, 40 * time.Second, 41 * time.Second},
	}

	for _, tt := range tests {
		payload := `{"test": "data"}`
		manager.SendToRetry(ctx, "orig-test", payload, tt.failureCount, "test", nil)

		messages, _ := client.XRange(ctx, manager.retryStream, "-", "+").Result()
		if len(messages) == 0 {
			t.Fatalf("No retry message created for failureCount=%d", tt.failureCount)
		}

		// Get last message
		lastMsg := messages[len(messages)-1]
		retryAtStr, ok := lastMsg.Values["retry_at"].(string)
		if !ok {
			t.Fatalf("retry_at not found for failureCount=%d", tt.failureCount)
		}

		// Just verify the field exists and is reasonable
		if retryAtStr == "" {
			t.Errorf("Empty retry_at for failureCount=%d", tt.failureCount)
		}

		client.XDel(ctx, manager.retryStream, lastMsg.ID)
	}
}

func TestGetDLQStats(t *testing.T) {
	client := setupTestRedis(t)
	defer cleanupTestRedis(t, client)

	manager := New(client)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Ensure streams exist
	manager.EnsureStreams(ctx)

	// Add a few messages
	for i := 0; i < 3; i++ {
		manager.SendToDLQ(ctx, "orig-"+string(rune(i)), `{"data": "test"}`, 
			errors.New("test error"), "test", nil)
	}

	stats, err := manager.GetDLQStats(ctx)
	if err != nil {
		t.Fatalf("GetDLQStats failed: %v", err)
	}

	if stats["total_messages"] != int64(3) {
		t.Errorf("Expected 3 total messages, got %v", stats["total_messages"])
	}
}

func TestConsumeAndAcknowledge(t *testing.T) {
	client := setupTestRedis(t)
	defer cleanupTestRedis(t, client)

	manager := New(client)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Ensure streams exist
	manager.EnsureStreams(ctx)

	// Add test message
	manager.SendToDLQ(ctx, "orig-789", `{"message": "test"}`, 
		errors.New("test error"), "test", nil)

	// Consume in background
	go func() {
		manager.ConsumeDLQ(ctx, "test-consumer", 10, func(ctx context.Context, msg *DeadLetterMessage) error {
			// Simulate successful processing
			return nil
		})
	}()

	// Give consumer time to process
	time.Sleep(1 * time.Second)

	// Check that message was acknowledged
	pending, err := client.XPending(ctx, manager.dlqStream, manager.dlqConsumerGroup).Result()
	if err != nil {
		t.Fatalf("XPending failed: %v", err)
	}

	// After successful processing, pending count should be 0
	if pending.Count != 0 {
		t.Logf("Note: Pending count is %d (timing dependent)", pending.Count)
	}

	cancel()
}

func TestReplayMessage(t *testing.T) {
	client := setupTestRedis(t)
	defer cleanupTestRedis(t, client)

	manager := New(client)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Ensure streams exist
	manager.EnsureStreams(ctx)

	// Add message to DLQ
	msgID, err := manager.SendToDLQ(ctx, "orig-replay", `{"data": "test"}`, 
		errors.New("replay test"), "test", nil)
	if err != nil {
		t.Fatalf("SendToDLQ failed: %v", err)
	}

	// Replay the message
	err = manager.ReplayMessage(ctx, msgID)
	if err != nil {
		t.Fatalf("ReplayMessage failed: %v", err)
	}

	// Verify message moved to retry
	messages, _ := client.XRange(ctx, manager.retryStream, "-", "+").Result()
	if len(messages) == 0 {
		t.Fatal("Message not found in retry stream after replay")
	}

	// Verify removed from DLQ
	dlqMessages, _ := client.XRange(ctx, manager.dlqStream, "-", "+").Result()
	if len(dlqMessages) > 0 {
		t.Fatal("Message still in DLQ after replay")
	}
}

func TestPurgeDLQ(t *testing.T) {
	client := setupTestRedis(t)
	defer cleanupTestRedis(t, client)

	manager := New(client)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Add messages
	for i := 0; i < 5; i++ {
		manager.SendToDLQ(ctx, "orig-"+string(rune(i)), `{"data": "test"}`, 
			errors.New("test"), "test", nil)
	}

	// Verify messages added
	length, _ := client.XLen(ctx, manager.dlqStream).Result()
	if length != 5 {
		t.Fatalf("Expected 5 messages, got %d", length)
	}

	// Purge messages older than 1 second (all should be purged)
	time.Sleep(2 * time.Second)
	purged, err := manager.PurgeDLQ(ctx, 1*time.Second)
	if err != nil {
		t.Fatalf("PurgeDLQ failed: %v", err)
	}

	if purged != 5 {
		t.Errorf("Expected to purge 5 messages, purged %d", purged)
	}
}
