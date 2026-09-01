package health

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusDegraded  Status = "degraded"
	StatusUnhealthy Status = "unhealthy"
)

// ServiceHealth represents health status of a service
type ServiceHealth struct {
	Status      Status            `json:"status"`
	Message     string            `json:"message"`
	ResponseTime int64            `json:"response_time_ms"`
	Checks      map[string]string `json:"checks"`
}

// HealthChecker performs health checks on backend services
type HealthChecker struct {
	postgres *pgxpool.Pool
	redis    *redis.Client
}

// New creates a new health checker
func New(postgres *pgxpool.Pool, redis *redis.Client) *HealthChecker {
	return &HealthChecker{
		postgres: postgres,
		redis:    redis,
	}
}

// CheckPostgres checks PostgreSQL connectivity and basic health
func (h *HealthChecker) CheckPostgres(ctx context.Context) ServiceHealth {
	start := time.Now()
	checks := make(map[string]string)

	// Check connection
	conn, err := h.postgres.Acquire(ctx)
	if err != nil {
		return ServiceHealth{
			Status:       StatusUnhealthy,
			Message:      fmt.Sprintf("Cannot acquire connection: %v", err),
			ResponseTime: time.Since(start).Milliseconds(),
			Checks:       checks,
		}
	}
	defer conn.Release()

	checks["connection"] = "ok"

	// Check database is accessible
	var version string
	err = conn.QueryRow(ctx, "SELECT version()").Scan(&version)
	if err != nil {
		return ServiceHealth{
			Status:       StatusUnhealthy,
			Message:      fmt.Sprintf("Query failed: %v", err),
			ResponseTime: time.Since(start).Milliseconds(),
			Checks:       checks,
		}
	}

	checks["query"] = "ok"

	// Check tables exist (api_keys, assets, users, etc.)
	var tableCount int
	err = conn.QueryRow(ctx, `
		SELECT COUNT(*) FROM information_schema.tables 
		WHERE table_schema='public' AND table_type='BASE TABLE'
	`).Scan(&tableCount)
	if err != nil || tableCount < 5 {
		return ServiceHealth{
			Status:       StatusDegraded,
			Message:      fmt.Sprintf("Expected tables not found: %d found", tableCount),
			ResponseTime: time.Since(start).Milliseconds(),
			Checks:       checks,
		}
	}

	checks["tables"] = fmt.Sprintf("ok (%d tables)", tableCount)

	return ServiceHealth{
		Status:       StatusHealthy,
		Message:      "PostgreSQL is healthy",
		ResponseTime: time.Since(start).Milliseconds(),
		Checks:       checks,
	}
}

// CheckRedis checks Redis connectivity and basic operations
func (h *HealthChecker) CheckRedis(ctx context.Context) ServiceHealth {
	start := time.Now()
	checks := make(map[string]string)

	// Check PING
	err := h.redis.Ping(ctx).Err()
	if err != nil {
		return ServiceHealth{
			Status:       StatusUnhealthy,
			Message:      fmt.Sprintf("PING failed: %v", err),
			ResponseTime: time.Since(start).Milliseconds(),
			Checks:       checks,
		}
	}

	checks["ping"] = "ok"

	// Check we can write/read
	testKey := "health:test"
	testValue := "health_check_" + fmt.Sprintf("%d", time.Now().UnixNano())

	err = h.redis.Set(ctx, testKey, testValue, 1*time.Second).Err()
	if err != nil {
		return ServiceHealth{
			Status:       StatusUnhealthy,
			Message:      fmt.Sprintf("SET failed: %v", err),
			ResponseTime: time.Since(start).Milliseconds(),
			Checks:       checks,
		}
	}

	val, err := h.redis.Get(ctx, testKey).Result()
	if err != nil || val != testValue {
		return ServiceHealth{
			Status:       StatusUnhealthy,
			Message:      fmt.Sprintf("GET failed: %v", err),
			ResponseTime: time.Since(start).Milliseconds(),
			Checks:       checks,
		}
	}

	checks["read_write"] = "ok"

	// Check memory usage
	info := h.redis.Info(ctx, "memory")
	if info.Err() == nil {
		checks["memory"] = "ok"
	}

	return ServiceHealth{
		Status:       StatusHealthy,
		Message:      "Redis is healthy",
		ResponseTime: time.Since(start).Milliseconds(),
		Checks:       checks,
	}
}

// CheckQueueHealth checks ingest/parser queue health
func (h *HealthChecker) CheckQueueHealth(ctx context.Context) ServiceHealth {
	start := time.Now()
	checks := make(map[string]string)

	// Check ingest stream exists
	ingestLen, err := h.redis.XLen(ctx, "siem:ingest").Result()
	if err != nil {
		checks["ingest_stream"] = fmt.Sprintf("error: %v", err)
	} else {
		checks["ingest_stream"] = fmt.Sprintf("ok (%d messages)", ingestLen)
	}

	// Check parser stream
	parserLen, err := h.redis.XLen(ctx, "siem:parser").Result()
	if err != nil {
		checks["parser_stream"] = fmt.Sprintf("error: %v", err)
	} else {
		checks["parser_stream"] = fmt.Sprintf("ok (%d messages)", parserLen)
	}

	// Check retry queue
	retryLen, err := h.redis.XLen(ctx, "siem:retry-queue").Result()
	if err != nil {
		checks["retry_queue"] = fmt.Sprintf("error: %v", err)
	} else {
		checks["retry_queue"] = fmt.Sprintf("ok (%d messages)", retryLen)
		// Warn if retry queue is growing
		if retryLen > 100 {
			return ServiceHealth{
				Status:       StatusDegraded,
				Message:      fmt.Sprintf("Retry queue has %d messages (threshold: 100)", retryLen),
				ResponseTime: time.Since(start).Milliseconds(),
				Checks:       checks,
			}
		}
	}

	// Check DLQ
	dlqLen, err := h.redis.XLen(ctx, "siem:dead-letter-queue").Result()
	if err != nil {
		checks["dlq"] = fmt.Sprintf("error: %v", err)
	} else {
		checks["dlq"] = fmt.Sprintf("ok (%d messages)", dlqLen)
		// Warn if DLQ is growing
		if dlqLen > 10 {
			return ServiceHealth{
				Status:       StatusDegraded,
				Message:      fmt.Sprintf("DLQ has %d messages (threshold: 10)", dlqLen),
				ResponseTime: time.Since(start).Milliseconds(),
				Checks:       checks,
			}
		}
	}

	return ServiceHealth{
		Status:       StatusHealthy,
		Message:      "Queue health is ok",
		ResponseTime: time.Since(start).Milliseconds(),
		Checks:       checks,
	}
}

// CheckConnectorHealth checks ingest/parser services connectivity
func (h *HealthChecker) CheckConnectorHealth(ctx context.Context) ServiceHealth {
	start := time.Now()
	checks := make(map[string]string)

	// Check if ingest consumer group exists
	groups, err := h.redis.XInfoGroups(ctx, "siem:ingest").Result()
	if err != nil {
		checks["ingest_group"] = fmt.Sprintf("error: %v", err)
	} else if len(groups) == 0 {
		checks["ingest_group"] = "not found (waiting for connection)"
	} else {
		checks["ingest_group"] = fmt.Sprintf("ok (%d consumers)", groups[0].Consumers)
	}

	// Check if parser consumer group exists
	pGroups, err := h.redis.XInfoGroups(ctx, "siem:parser").Result()
	if err != nil {
		checks["parser_group"] = fmt.Sprintf("error: %v", err)
	} else if len(pGroups) == 0 {
		checks["parser_group"] = "not found (waiting for connection)"
	} else {
		checks["parser_group"] = fmt.Sprintf("ok (%d consumers)", pGroups[0].Consumers)
	}

	status := StatusHealthy
	message := "Connectors are ready"

	// If no consumers connected, status is degraded
	if len(groups) == 0 || len(pGroups) == 0 {
		status = StatusDegraded
		message = "Waiting for ingest/parser services to connect"
	}

	return ServiceHealth{
		Status:       status,
		Message:      message,
		ResponseTime: time.Since(start).Milliseconds(),
		Checks:       checks,
	}
}

// CheckDatabase checks database connectivity and migration status
func (h *HealthChecker) CheckDatabase(ctx context.Context) ServiceHealth {
	start := time.Now()
	checks := make(map[string]string)

	conn, err := h.postgres.Acquire(ctx)
	if err != nil {
		return ServiceHealth{
			Status:       StatusUnhealthy,
			Message:      fmt.Sprintf("Cannot acquire connection: %v", err),
			ResponseTime: time.Since(start).Milliseconds(),
			Checks:       checks,
		}
	}
	defer conn.Release()

	// Check migration status
	var lastMigration string
	err = conn.QueryRow(ctx, `
		SELECT name FROM schema_migrations 
		ORDER BY name DESC LIMIT 1
	`).Scan(&lastMigration)
	if err != nil && err != sql.ErrNoRows {
		return ServiceHealth{
			Status:       StatusDegraded,
			Message:      fmt.Sprintf("Migration check failed: %v", err),
			ResponseTime: time.Since(start).Milliseconds(),
			Checks:       checks,
		}
	}

	if lastMigration == "" {
		checks["migrations"] = "no migrations applied"
	} else {
		checks["migrations"] = fmt.Sprintf("ok (latest: %s)", lastMigration)
	}

	// Check critical tables
	requiredTables := []string{"users", "assets", "api_keys", "rules", "alerts"}
	for _, table := range requiredTables {
		var exists bool
		err := conn.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM information_schema.tables 
				WHERE table_schema='public' AND table_name=$1
			)
		`, table).Scan(&exists)
		if err != nil || !exists {
			checks[table] = "missing"
			return ServiceHealth{
				Status:       StatusUnhealthy,
				Message:      fmt.Sprintf("Required table %s not found", table),
				ResponseTime: time.Since(start).Milliseconds(),
				Checks:       checks,
			}
		}
		checks[table] = "ok"
	}

	return ServiceHealth{
		Status:       StatusHealthy,
		Message:      "Database schema is ready",
		ResponseTime: time.Since(start).Milliseconds(),
		Checks:       checks,
	}
}

// CheckAll performs all health checks
func (h *HealthChecker) CheckAll(ctx context.Context) map[string]ServiceHealth {
	return map[string]ServiceHealth{
		"postgres":   h.CheckPostgres(ctx),
		"redis":      h.CheckRedis(ctx),
		"database":   h.CheckDatabase(ctx),
		"queues":     h.CheckQueueHealth(ctx),
		"connectors": h.CheckConnectorHealth(ctx),
	}
}

// IsHealthy returns true if all critical services are healthy
func (h *HealthChecker) IsHealthy(ctx context.Context) bool {
	checks := h.CheckAll(ctx)
	for _, check := range checks {
		if check.Status == StatusUnhealthy {
			return false
		}
	}
	return true
}

// OverallStatus returns the worst status from all checks
func (h *HealthChecker) OverallStatus(ctx context.Context) Status {
	checks := h.CheckAll(ctx)
	hasUnhealthy := false
	hasDegraded := false

	for _, check := range checks {
		if check.Status == StatusUnhealthy {
			hasUnhealthy = true
		} else if check.Status == StatusDegraded {
			hasDegraded = true
		}
	}

	if hasUnhealthy {
		return StatusUnhealthy
	}
	if hasDegraded {
		return StatusDegraded
	}
	return StatusHealthy
}
