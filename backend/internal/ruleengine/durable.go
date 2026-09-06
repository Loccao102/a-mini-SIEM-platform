package ruleengine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/parser"
	"github.com/jackc/pgx/v5"
)

// recordAlert commits the event receipt and alert together. A replay after an
// indexing success or process crash cannot count the same rule/event twice.
func (engine *Engine) recordAlert(ctx context.Context, rule Rule, event parser.NormalizedEvent) error {
	if event.EventID == "" {
		return fmt.Errorf("event ID is required")
	}
	entity := event.Hostname + ":" + getEntityKey(event)
	tx, err := engine.postgres.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, fmt.Sprintf("rule:%d:%s", rule.ID, entity)); err != nil {
		return err
	}
	inserted, err := tx.Exec(ctx, `INSERT INTO rule_event_receipts(rule_id,event_id,event_time,entity_key) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, rule.ID, event.EventID, event.EventTime, entity)
	if err != nil {
		return err
	}
	if inserted.RowsAffected() == 0 {
		return tx.Commit(ctx)
	}
	var condition struct {
		Count    int `json:"count"`
		Window   int `json:"window_seconds"`
		Cooldown int `json:"cooldown_seconds"`
	}
	if len(rule.Condition) > 0 {
		if err = json.Unmarshal(rule.Condition, &condition); err != nil {
			return fmt.Errorf("rule %d condition: %w", rule.ID, err)
		}
	}
	if condition.Window <= 0 {
		condition.Window = 60
	}
	if condition.Cooldown <= 0 {
		condition.Cooldown = 180
	}
	if condition.Count > 1 {
		var count int
		if err = tx.QueryRow(ctx, `SELECT count(*) FROM rule_event_receipts WHERE rule_id=$1 AND entity_key=$2 AND observed_at>=now()-make_interval(secs=>$3)`, rule.ID, entity, condition.Window).Scan(&count); err != nil {
			return err
		}
		if count < condition.Count {
			return tx.Commit(ctx)
		}
	}
	var alertID int64
	err = tx.QueryRow(ctx, `SELECT alert_id FROM alerts WHERE rule_id=$1 AND entity_key=$2 AND status IN ('open','acknowledged') AND last_seen>=now()-make_interval(secs=>$3) ORDER BY last_seen DESC LIMIT 1 FOR UPDATE`, rule.ID, entity, condition.Cooldown).Scan(&alertID)
	isNew := errors.Is(err, pgx.ErrNoRows)
	if err != nil && !isNew {
		return err
	}
	if isNew {
		err = tx.QueryRow(ctx, `INSERT INTO alerts(rule_id,severity,summary,entity_key,occurrences) VALUES($1,$2,$3,$4,1) RETURNING alert_id`, rule.ID, rule.Severity, rule.Name+": "+event.Message, entity).Scan(&alertID)
	} else {
		_, err = tx.Exec(ctx, `UPDATE alerts SET occurrences=occurrences+1,last_seen=now() WHERE alert_id=$1`, alertID)
	}
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO alert_events(alert_id,event_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, alertID, event.EventID); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	// Notifications are best effort; database state is authoritative.
	if isNew && engine.notifier != nil {
		if err = engine.notifier.Send(ctx, fmt.Sprintf("[%s] %s: %s", rule.Severity, rule.Name, event.Message)); err != nil {
			fmt.Printf("notification: %v\n", err)
		}
	}
	return nil
}
