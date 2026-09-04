package health

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/storage"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusDegraded  Status = "degraded"
	StatusUnhealthy Status = "unhealthy"

	RetryQueueAlertThreshold int64 = 100
	DLQAlertThreshold        int64 = 10
)

// ServiceHealth represents health status of a service
type ServiceHealth struct {
	Status       Status            `json:"status"`
	Message      string            `json:"message"`
	ResponseTime int64             `json:"response_time_ms"`
	Checks       map[string]string `json:"checks"`
}

// HealthChecker performs health checks on backend services
type HealthChecker struct {
	postgres *pgxpool.Pool
	redis    *redis.Client
	elastic  *storage.Elasticsearch
}

// New creates a new health checker
func New(postgres *pgxpool.Pool, redis *redis.Client, elastic ...*storage.Elasticsearch) *HealthChecker {
	checker := &HealthChecker{
		postgres: postgres,
		redis:    redis,
	}
	if len(elastic) > 0 {
		checker.elastic = elastic[0]
	}
	return checker
}

func (h *HealthChecker) CheckElasticsearch(ctx context.Context) ServiceHealth {
	start := time.Now()
	if h.elastic == nil {
		return ServiceHealth{Status: StatusUnhealthy, Message: "Elasticsearch client is not configured", ResponseTime: time.Since(start).Milliseconds(), Checks: map[string]string{}}
	}
	if err := h.elastic.ClusterHealth(ctx); err != nil {
		return ServiceHealth{Status: StatusUnhealthy, Message: fmt.Sprintf("Elasticsearch check failed: %v", err), ResponseTime: time.Since(start).Milliseconds(), Checks: map[string]string{"cluster": "error"}}
	}
	return ServiceHealth{Status: StatusHealthy, Message: "Elasticsearch is healthy", ResponseTime: time.Since(start).Milliseconds(), Checks: map[string]string{"cluster": "ok"}}
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
	status := StatusHealthy
	message := "Queue health is ok"

	// The API publishes to raw-logs and the parser consumes the same stream.
	ingestLen, err := h.redis.XLen(ctx, "siem:raw-logs").Result()
	if err != nil {
		checks["ingest_stream"] = fmt.Sprintf("error: %v", err)
		status = StatusUnhealthy
		message = "Ingest stream cannot be read"
	} else {
		checks["ingest_stream"] = fmt.Sprintf("ok (%d messages)", ingestLen)
	}

	checks["parser_stream"] = checks["ingest_stream"]

	// Check retry queue
	retryLen, err := h.redis.XLen(ctx, "siem:retry-queue").Result()
	if err != nil {
		checks["retry_queue"] = fmt.Sprintf("error: %v", err)
		status = StatusUnhealthy
		message = "Retry queue cannot be read"
	} else {
		checks["retry_queue"] = fmt.Sprintf("ok (%d messages)", retryLen)
		if retryLen >= RetryQueueAlertThreshold {
			checks["retry_queue"] = fmt.Sprintf("alert: queue_full (%d/%d messages)", retryLen, RetryQueueAlertThreshold)
			status = StatusUnhealthy
			message = fmt.Sprintf("Retry queue is full: %d messages (threshold: %d)", retryLen, RetryQueueAlertThreshold)
			h.recordPipelineAlert(ctx, "retry_queue", "critical", retryLen, RetryQueueAlertThreshold, message)
		} else {
			h.resolvePipelineAlert(ctx, "retry_queue")
		}
	}

	// Check DLQ
	dlqLen, err := h.redis.XLen(ctx, "siem:dead-letter-queue").Result()
	if err != nil {
		checks["dlq"] = fmt.Sprintf("error: %v", err)
		status = StatusUnhealthy
		message = "Dead-letter queue cannot be read"
	} else {
		checks["dlq"] = fmt.Sprintf("ok (%d messages)", dlqLen)
		if dlqLen >= DLQAlertThreshold {
			checks["dlq"] = fmt.Sprintf("alert: queue_full (%d/%d messages)", dlqLen, DLQAlertThreshold)
			if status != StatusUnhealthy {
				status = StatusDegraded
				message = fmt.Sprintf("DLQ requires attention: %d messages (threshold: %d)", dlqLen, DLQAlertThreshold)
			}
			h.recordPipelineAlert(ctx, "dead_letter_queue", "warning", dlqLen, DLQAlertThreshold, "Dead-letter queue requires attention")
		} else {
			h.resolvePipelineAlert(ctx, "dead_letter_queue")
		}
	}

	return ServiceHealth{
		Status:       status,
		Message:      message,
		ResponseTime: time.Since(start).Milliseconds(),
		Checks:       checks,
	}
}

func (h *HealthChecker) recordPipelineAlert(ctx context.Context, queueName, severity string, observed, threshold int64, message string) {
	if h.postgres == nil {
		return
	}
	_, _ = h.postgres.Exec(ctx, `
		INSERT INTO pipeline_alerts (alert_key, severity, status, queue_name, observed_value, threshold, message)
		VALUES ($1, $2, 'open', $3, $4, $5, $6)
		ON CONFLICT (alert_key) DO UPDATE SET
			severity = EXCLUDED.severity, status = 'open', observed_value = EXCLUDED.observed_value,
			threshold = EXCLUDED.threshold, message = EXCLUDED.message, last_seen = now(), resolved_at = NULL
	`, "queue:"+queueName, severity, queueName, observed, threshold, message)
}

func (h *HealthChecker) resolvePipelineAlert(ctx context.Context, queueName string) {
	if h.postgres == nil {
		return
	}
	_, _ = h.postgres.Exec(ctx, `UPDATE pipeline_alerts SET status='resolved', resolved_at=COALESCE(resolved_at, now()), last_seen=now() WHERE alert_key=$1 AND status='open'`, "queue:"+queueName)
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
		"postgres":      h.CheckPostgres(ctx),
		"redis":         h.CheckRedis(ctx),
		"elasticsearch": h.CheckElasticsearch(ctx),
		"database":      h.CheckDatabase(ctx),
		"queues":        h.CheckQueueHealth(ctx),
		"connectors":    h.CheckConnectorHealth(ctx),
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
