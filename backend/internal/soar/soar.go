package soar

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Playbook struct {
	ID               int64           `json:"playbook_id"`
	Name             string          `json:"name"`
	Description      string          `json:"description"`
	TriggerType      string          `json:"trigger_type"` // severity, category, rule_match
	TriggerFilter    json.RawMessage `json:"trigger_filter"`
	ActionType       string          `json:"action_type"` // block_ip, isolate_user, notify_telegram
	ActionParams     json.RawMessage `json:"action_params"`
	RequiresApproval bool            `json:"requires_approval"`
	Enabled          bool            `json:"enabled"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

type Execution struct {
	ID         int64           `json:"execution_id"`
	PlaybookID *int64          `json:"playbook_id,omitempty"`
	AlertID    *int64          `json:"alert_id,omitempty"`
	ActionType string          `json:"action_type"`
	Target     string          `json:"target"`
	Status     string          `json:"status"` // pending_approval, approved, executed, rejected, failed, rolled_back
	Output     json.RawMessage `json:"output"`
	ApprovedBy *int64          `json:"approved_by,omitempty"`
	ExpiresAt  *time.Time      `json:"expires_at,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
	ExecutedAt *time.Time      `json:"executed_at,omitempty"`
}

type BlockedEntity struct {
	ID          int64      `json:"blocked_id"`
	EntityType  string     `json:"entity_type"` // ip, user
	EntityValue string     `json:"entity_value"`
	Reason      string     `json:"reason"`
	ExecutionID *int64     `json:"execution_id,omitempty"`
	Status      string     `json:"status"` // active, released
	BlockedAt   time.Time  `json:"blocked_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	ReleasedAt  *time.Time `json:"released_at,omitempty"`
	ReleasedBy  *int64     `json:"released_by,omitempty"`
}

type Engine struct {
	db        *pgxpool.Pool
	mu        sync.RWMutex
	allowlist map[string]bool
}

func NewEngine(db *pgxpool.Pool, customAllowlist []string) *Engine {
	al := map[string]bool{
		"127.0.0.1":     true,
		"::1":           true,
		"localhost":     true,
		"0.0.0.0":       true,
		"192.168.1.1":   true,
		"admin":         true,
		"root":          true,
		"administrator": true,
	}
	for _, item := range customAllowlist {
		clean := strings.ToLower(strings.TrimSpace(item))
		if clean != "" {
			al[clean] = true
		}
	}
	return &Engine{
		db:        db,
		allowlist: al,
	}
}

// IsAllowlisted protects critical IPs and administrators from being accidentally locked out.
func (e *Engine) IsAllowlisted(target string) bool {
	target = strings.ToLower(strings.TrimSpace(target))
	if target == "" {
		return true
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.allowlist[target] {
		return true
	}
	if addr, err := netip.ParseAddr(target); err == nil {
		if addr.IsLoopback() || addr.IsUnspecified() {
			return true
		}
	}
	return false
}

// EvaluateAlert checks active playbooks against alert criteria and creates executions.
func (e *Engine) EvaluateAlert(ctx context.Context, alertID int64, severity, category, ruleName, targetIP, targetUser string) ([]Execution, error) {
	rows, err := e.db.Query(ctx, `
		SELECT playbook_id, name, trigger_type, trigger_filter, action_type, action_params, requires_approval, enabled
		FROM playbooks
		WHERE enabled = true
	`)
	if err != nil {
		return nil, fmt.Errorf("query playbooks: %w", err)
	}
	defer rows.Close()

	var matched []Playbook
	for rows.Next() {
		var p Playbook
		if err := rows.Scan(&p.ID, &p.Name, &p.TriggerType, &p.TriggerFilter, &p.ActionType, &p.ActionParams, &p.RequiresApproval, &p.Enabled); err != nil {
			return nil, err
		}

		var filter map[string]string
		if len(p.TriggerFilter) > 0 {
			_ = json.Unmarshal(p.TriggerFilter, &filter)
		}

		isMatch := false
		switch p.TriggerType {
		case "severity":
			if filter["severity"] == "" || strings.EqualFold(filter["severity"], severity) {
				isMatch = true
			}
		case "category":
			catMatch := filter["category"] == "" || strings.EqualFold(filter["category"], category)
			sevMatch := filter["severity"] == "" || strings.EqualFold(filter["severity"], severity)
			isMatch = catMatch && sevMatch
		case "rule_match":
			if filter["rule_name"] == "" || strings.EqualFold(filter["rule_name"], ruleName) {
				isMatch = true
			}
		}

		if isMatch {
			matched = append(matched, p)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	executions := make([]Execution, 0, len(matched))
	for _, p := range matched {
		var target string
		switch p.ActionType {
		case "block_ip":
			target = targetIP
		case "isolate_user":
			target = targetUser
		default:
			target = targetIP
			if target == "" {
				target = targetUser
			}
		}

		if strings.TrimSpace(target) == "" || e.IsAllowlisted(target) {
			continue // Protected by allowlist or empty target
		}

		var params struct {
			TTLSeconds int `json:"ttl_seconds"`
		}
		if len(p.ActionParams) > 0 {
			_ = json.Unmarshal(p.ActionParams, &params)
		}
		if params.TTLSeconds <= 0 {
			params.TTLSeconds = 3600 // Default 1 hour TTL
		}

		now := time.Now().UTC()
		expiresAt := now.Add(time.Duration(params.TTLSeconds) * time.Second)

		status := "pending_approval"
		var executedAt *time.Time
		output := map[string]any{
			"message":     fmt.Sprintf("Playbook '%s' triggered by alert %d", p.Name, alertID),
			"ttl_seconds": params.TTLSeconds,
		}

		if !p.RequiresApproval {
			status = "executed"
			executedAt = &now
			output["executed_immediately"] = true
		}

		outJSON, _ := json.Marshal(output)

		var execID int64
		err := e.db.QueryRow(ctx, `
			INSERT INTO playbook_executions (playbook_id, alert_id, action_type, target, status, output, expires_at, created_at, executed_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			RETURNING execution_id
		`, p.ID, alertID, p.ActionType, target, status, outJSON, expiresAt, now, executedAt).Scan(&execID)
		if err != nil {
			return nil, fmt.Errorf("create playbook execution: %w", err)
		}

		if status == "executed" {
			_ = e.applyAction(ctx, execID, p.ActionType, target, "Auto-executed by playbook: "+p.Name, &expiresAt)
		}

		executions = append(executions, Execution{
			ID:         execID,
			PlaybookID: &p.ID,
			AlertID:    &alertID,
			ActionType: p.ActionType,
			Target:     target,
			Status:     status,
			Output:     outJSON,
			ExpiresAt:  &expiresAt,
			CreatedAt:  now,
			ExecutedAt: executedAt,
		})
	}

	return executions, nil
}

// ApproveExecution approves and executes a pending action.
func (e *Engine) ApproveExecution(ctx context.Context, execID int64, approvedByUserID int64) error {
	tx, err := e.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var exec Execution
	err = tx.QueryRow(ctx, `
		SELECT execution_id, action_type, target, status, expires_at 
		FROM playbook_executions 
		WHERE execution_id = $1 FOR UPDATE
	`, execID).Scan(&exec.ID, &exec.ActionType, &exec.Target, &exec.Status, &exec.ExpiresAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("execution %d not found", execID)
		}
		return err
	}

	if exec.Status != "pending_approval" {
		return fmt.Errorf("execution is %s, can only approve pending_approval", exec.Status)
	}

	if e.IsAllowlisted(exec.Target) {
		return fmt.Errorf("target %q is protected by safety allowlist", exec.Target)
	}

	now := time.Now().UTC()
	_, err = tx.Exec(ctx, `
		UPDATE playbook_executions
		SET status = 'executed', approved_by = $1, executed_at = $2
		WHERE execution_id = $3
	`, approvedByUserID, now, execID)
	if err != nil {
		return err
	}

	// Apply mitigation in blocked_entities
	entityType := "ip"
	if exec.ActionType == "isolate_user" {
		entityType = "user"
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO blocked_entities (entity_type, entity_value, reason, execution_id, status, blocked_at, expires_at)
		VALUES ($1, $2, $3, $4, 'active', $5, $6)
	`, entityType, exec.Target, fmt.Sprintf("Approved SOAR execution #%d", execID), execID, now, exec.ExpiresAt)
	if err != nil {
		return err
	}

	// Log audit trail
	_, _ = tx.Exec(ctx, `
		INSERT INTO audit_logs (actor_user_id, action, entity_type, entity_id, details)
		VALUES ($1, 'soar.execution_approved', 'playbook_execution', $2, $3)
	`, approvedByUserID, execID, fmt.Sprintf(`{"target": "%s", "action": "%s"}`, exec.Target, exec.ActionType))

	return tx.Commit(ctx)
}

// RejectExecution marks a pending execution as rejected by the analyst.
func (e *Engine) RejectExecution(ctx context.Context, execID int64, rejectedByUserID int64) error {
	result, err := e.db.Exec(ctx, `
		UPDATE playbook_executions
		SET status = 'rejected', approved_by = $1, executed_at = now()
		WHERE execution_id = $2 AND status = 'pending_approval'
	`, rejectedByUserID, execID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("execution %d not found or not in pending_approval state", execID)
	}

	_, _ = e.db.Exec(ctx, `
		INSERT INTO audit_logs (actor_user_id, action, entity_type, entity_id, details)
		VALUES ($1, 'soar.execution_rejected', 'playbook_execution', $2, '{}')
	`, rejectedByUserID, execID)

	return nil
}

// ReleaseBlockedEntity manually unblocks an entity before its TTL expiration.
func (e *Engine) ReleaseBlockedEntity(ctx context.Context, blockedID int64, releasedByUserID int64) error {
	tx, err := e.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var val, typ string
	var execID *int64
	err = tx.QueryRow(ctx, `
		UPDATE blocked_entities
		SET status = 'released', released_at = now(), released_by = $1
		WHERE blocked_id = $2 AND status = 'active'
		RETURNING entity_value, entity_type, execution_id
	`, releasedByUserID, blockedID).Scan(&val, &typ, &execID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("active blocked entity %d not found", blockedID)
		}
		return err
	}

	if execID != nil {
		_, _ = tx.Exec(ctx, `UPDATE playbook_executions SET status = 'rolled_back' WHERE execution_id = $1`, *execID)
	}

	_, _ = tx.Exec(ctx, `
		INSERT INTO audit_logs (actor_user_id, action, entity_type, entity_id, details)
		VALUES ($1, 'soar.entity_released', 'blocked_entity', $2, $3)
	`, releasedByUserID, blockedID, fmt.Sprintf(`{"entity": "%s", "type": "%s"}`, val, typ))

	return tx.Commit(ctx)
}

// ReleaseExpired automatically rolls back expired blocked entities (TTL cleanup).
func (e *Engine) ReleaseExpired(ctx context.Context) (int64, error) {
	tag, err := e.db.Exec(ctx, `
		UPDATE blocked_entities
		SET status = 'released', released_at = now()
		WHERE status = 'active' AND expires_at IS NOT NULL AND expires_at <= now()
	`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (e *Engine) applyAction(ctx context.Context, execID int64, actionType, target, reason string, expiresAt *time.Time) error {
	entityType := "ip"
	if actionType == "isolate_user" {
		entityType = "user"
	}
	_, err := e.db.Exec(ctx, `
		INSERT INTO blocked_entities (entity_type, entity_value, reason, execution_id, status, blocked_at, expires_at)
		VALUES ($1, $2, $3, $4, 'active', now(), $5)
	`, entityType, target, reason, execID, expiresAt)
	return err
}

// Query Helpers for Handlers
func (e *Engine) ListPlaybooks(ctx context.Context) ([]Playbook, error) {
	rows, err := e.db.Query(ctx, `
		SELECT playbook_id, name, description, trigger_type, trigger_filter, action_type, action_params, requires_approval, enabled, created_at, updated_at
		FROM playbooks ORDER BY playbook_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Playbook
	for rows.Next() {
		var p Playbook
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.TriggerType, &p.TriggerFilter, &p.ActionType, &p.ActionParams, &p.RequiresApproval, &p.Enabled, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (e *Engine) ListExecutions(ctx context.Context, limit int) ([]Execution, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := e.db.Query(ctx, `
		SELECT execution_id, playbook_id, alert_id, action_type, target, status, output, approved_by, expires_at, created_at, executed_at
		FROM playbook_executions ORDER BY created_at DESC LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Execution
	for rows.Next() {
		var ex Execution
		if err := rows.Scan(&ex.ID, &ex.PlaybookID, &ex.AlertID, &ex.ActionType, &ex.Target, &ex.Status, &ex.Output, &ex.ApprovedBy, &ex.ExpiresAt, &ex.CreatedAt, &ex.ExecutedAt); err != nil {
			return nil, err
		}
		out = append(out, ex)
	}
	return out, rows.Err()
}

func (e *Engine) ListBlocked(ctx context.Context) ([]BlockedEntity, error) {
	rows, err := e.db.Query(ctx, `
		SELECT blocked_id, entity_type, entity_value, reason, execution_id, status, blocked_at, expires_at, released_at, released_by
		FROM blocked_entities WHERE status = 'active' ORDER BY blocked_at DESC LIMIT 100
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []BlockedEntity
	for rows.Next() {
		var b BlockedEntity
		if err := rows.Scan(&b.ID, &b.EntityType, &b.EntityValue, &b.Reason, &b.ExecutionID, &b.Status, &b.BlockedAt, &b.ExpiresAt, &b.ReleasedAt, &b.ReleasedBy); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}
