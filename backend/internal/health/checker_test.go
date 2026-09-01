package health

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func setupTestPostgres(t *testing.T) *pgxpool.Pool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	config, err := pgxpool.ParseConfig("postgres://siem:siem@localhost:5432/siem")
	if err != nil {
		t.Skipf("PostgreSQL not available: %v", err)
	}

	pool, err := pgxpool.NewWithContext(ctx, config.ConnString())
	if err != nil {
		t.Skipf("Cannot connect to PostgreSQL: %v", err)
	}

	if err := pool.Ping(ctx); err != nil {
		t.Skipf("PostgreSQL not responding: %v", err)
	}

	return pool
}

func setupTestRedis(t *testing.T) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("Redis not available: %v", err)
	}

	return client
}

func TestCheckPostgres(t *testing.T) {
	pool := setupTestPostgres(t)
	defer pool.Close()

	client := setupTestRedis(t)
	defer client.Close()

	checker := New(pool, client)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	health := checker.CheckPostgres(ctx)
	if health.Status != StatusHealthy {
		t.Errorf("Expected healthy, got %v: %s", health.Status, health.Message)
	}

	if health.ResponseTime <= 0 {
		t.Errorf("Expected positive response time, got %d", health.ResponseTime)
	}
}

func TestCheckRedis(t *testing.T) {
	pool := setupTestPostgres(t)
	defer pool.Close()

	client := setupTestRedis(t)
	defer client.Close()

	checker := New(pool, client)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	health := checker.CheckRedis(ctx)
	if health.Status != StatusHealthy {
		t.Errorf("Expected healthy, got %v: %s", health.Status, health.Message)
	}

	if health.ResponseTime <= 0 {
		t.Errorf("Expected positive response time, got %d", health.ResponseTime)
	}
}

func TestCheckQueueHealth(t *testing.T) {
	pool := setupTestPostgres(t)
	defer pool.Close()

	client := setupTestRedis(t)
	defer client.Close()

	checker := New(pool, client)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	health := checker.CheckQueueHealth(ctx)
	// May be degraded or healthy depending on queue state
	if health.Status == StatusUnhealthy {
		t.Errorf("Queue health should not be unhealthy: %s", health.Message)
	}
}

func TestCheckDatabase(t *testing.T) {
	pool := setupTestPostgres(t)
	defer pool.Close()

	client := setupTestRedis(t)
	defer client.Close()

	checker := New(pool, client)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	health := checker.CheckDatabase(ctx)
	if health.Status == StatusUnhealthy {
		t.Errorf("Database check failed: %s", health.Message)
	}
}

func TestCheckAll(t *testing.T) {
	pool := setupTestPostgres(t)
	defer pool.Close()

	client := setupTestRedis(t)
	defer client.Close()

	checker := New(pool, client)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	allChecks := checker.CheckAll(ctx)
	if len(allChecks) == 0 {
		t.Fatal("CheckAll returned no checks")
	}

	// Verify expected checks exist
	expectedChecks := []string{"postgres", "redis", "database", "queues", "connectors"}
	for _, expected := range expectedChecks {
		if _, ok := allChecks[expected]; !ok {
			t.Errorf("Missing check: %s", expected)
		}
	}
}

func TestIsHealthy(t *testing.T) {
	pool := setupTestPostgres(t)
	defer pool.Close()

	client := setupTestRedis(t)
	defer client.Close()

	checker := New(pool, client)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Should be healthy if all services are up
	healthy := checker.IsHealthy(ctx)
	if !healthy {
		t.Log("System is not fully healthy (expected if services are degraded)")
	}
}

func TestOverallStatus(t *testing.T) {
	pool := setupTestPostgres(t)
	defer pool.Close()

	client := setupTestRedis(t)
	defer client.Close()

	checker := New(pool, client)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	status := checker.OverallStatus(ctx)
	if status == "" {
		t.Error("OverallStatus returned empty string")
	}

	// Status should be one of the valid values
	if status != StatusHealthy && status != StatusDegraded && status != StatusUnhealthy {
		t.Errorf("Invalid status: %s", status)
	}
}
