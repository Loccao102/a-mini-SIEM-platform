package ratelimit

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func setupTestRedis(t *testing.T) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("redis connection failed: %v", err)
	}

	// Clean up any test keys
	client.FlushDB(ctx)

	return client
}

func TestAllowSuccess(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()

	limiter := New(client, 10, 1*time.Minute)
	ctx := context.Background()

	allowed, remaining, _, err := limiter.Allow(ctx, "test-user-1")
	if err != nil {
		t.Fatalf("Allow failed: %v", err)
	}

	if !allowed {
		t.Error("First request should be allowed")
	}

	if remaining != 9 {
		t.Errorf("Remaining should be 9, got %d", remaining)
	}
}

func TestAllowMultipleRequests(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()

	limiter := New(client, 5, 1*time.Minute)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		allowed, remaining, _, err := limiter.Allow(ctx, "test-user-2")
		if err != nil {
			t.Fatalf("Allow failed on iteration %d: %v", i, err)
		}

		if !allowed {
			t.Errorf("Request %d should be allowed", i+1)
		}

		expected := int64(4 - i)
		if remaining != expected {
			t.Errorf("Iteration %d: remaining should be %d, got %d", i, expected, remaining)
		}
	}

	// 6th request should be rejected
	allowed, _, _, err := limiter.Allow(ctx, "test-user-2")
	if err != nil {
		t.Fatalf("Allow failed on 6th request: %v", err)
	}

	if allowed {
		t.Error("6th request should be rejected (rate limit exceeded)")
	}
}

func TestAllowDifferentIdentifiers(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()

	limiter := New(client, 2, 1*time.Minute)
	ctx := context.Background()

	// User 1 makes 2 requests
	for i := 0; i < 2; i++ {
		allowed, _, _, err := limiter.Allow(ctx, "user-A")
		if err != nil {
			t.Fatalf("Allow failed for user-A: %v", err)
		}
		if !allowed {
			t.Errorf("User-A request %d should be allowed", i+1)
		}
	}

	// User 2 should still have quota
	allowed, remaining, _, err := limiter.Allow(ctx, "user-B")
	if err != nil {
		t.Fatalf("Allow failed for user-B: %v", err)
	}

	if !allowed {
		t.Error("User-B should have independent rate limit")
	}

	if remaining != 1 {
		t.Errorf("User-B remaining should be 1, got %d", remaining)
	}
}

func TestResetLimit(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()

	limiter := New(client, 3, 1*time.Minute)
	ctx := context.Background()

	// Use up the limit
	for i := 0; i < 3; i++ {
		limiter.Allow(ctx, "test-user-3")
	}

	// Should be rate limited
	allowed, _, _, _ := limiter.Allow(ctx, "test-user-3")
	if allowed {
		t.Error("Should be rate limited before reset")
	}

	// Reset
	err := limiter.Reset(ctx, "test-user-3")
	if err != nil {
		t.Fatalf("Reset failed: %v", err)
	}

	// Should be allowed again
	allowed, _, _, err = limiter.Allow(ctx, "test-user-3")
	if err != nil {
		t.Fatalf("Allow after reset failed: %v", err)
	}

	if !allowed {
		t.Error("Should be allowed after reset")
	}
}

func TestGetStatus(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()

	limiter := New(client, 10, 1*time.Minute)
	ctx := context.Background()

	// Initial status
	current, limit, _, err := limiter.GetStatus(ctx, "test-user-4")
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	if current != 0 {
		t.Errorf("Initial current should be 0, got %d", current)
	}

	if limit != 10 {
		t.Errorf("Limit should be 10, got %d", limit)
	}

	// Make a request
	limiter.Allow(ctx, "test-user-4")

	// Check status again
	current, _, _, err = limiter.GetStatus(ctx, "test-user-4")
	if err != nil {
		t.Fatalf("GetStatus after request failed: %v", err)
	}

	if current != 1 {
		t.Errorf("Current after request should be 1, got %d", current)
	}
}

func TestRateLimitHeaders(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()

	limiter := New(client, 5, 1*time.Minute)
	ctx := context.Background()

	allowed, remaining, resetAt, err := limiter.Allow(ctx, "test-user-5")
	if err != nil {
		t.Fatalf("Allow failed: %v", err)
	}

	if !allowed {
		t.Error("Request should be allowed")
	}

	if remaining != 4 {
		t.Errorf("Remaining should be 4, got %d", remaining)
	}

	// Reset time should be in the future
	now := time.Now()
	if resetAt.Before(now) {
		t.Error("Reset time should be in the future")
	}

	if resetAt.After(now.Add(2 * time.Minute)) {
		t.Error("Reset time should be within the window")
	}
}
