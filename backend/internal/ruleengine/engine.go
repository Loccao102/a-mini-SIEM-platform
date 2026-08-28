package ruleengine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/parser"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type Engine struct {
	postgres *pgxpool.Pool
	redis    redis.UniversalClient
	notifier *TelegramNotifier
}

type Rule struct {
	ID                                   int64
	Name, Pattern, TargetField, Severity string
	Enabled                              bool
	Condition                            json.RawMessage
}

func New(postgres *pgxpool.Pool, redisURL, telegramToken, telegramChatID string) (*Engine, error) {
	options, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	return &Engine{
		postgres: postgres,
		redis:    redis.NewClient(options),
		notifier: NewTelegramNotifier(telegramToken, telegramChatID),
	}, nil
}

func (engine *Engine) Close() error {
	if client, ok := engine.redis.(*redis.Client); ok {
		return client.Close()
	}
	return nil
}

func (engine *Engine) Process(ctx context.Context, event parser.NormalizedEvent) error {
	rows, err := engine.postgres.Query(ctx, `SELECT rule_id, name, regex_pattern, target_field, severity, enabled, condition FROM rules WHERE enabled = true`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var rule Rule
		if err := rows.Scan(&rule.ID, &rule.Name, &rule.Pattern, &rule.TargetField, &rule.Severity, &rule.Enabled, &rule.Condition); err != nil {
			return err
		}

		matched, err := regexp.MatchString(rule.Pattern, fieldValue(event, rule.TargetField))
		if err != nil {
			return fmt.Errorf("invalid regex in rule %d: %w", rule.ID, err)
		}

		if matched && engine.shouldAlert(ctx, rule, event) {
			if err := engine.createOrAggregateAlert(ctx, rule, event); err != nil {
				return err
			}
		}
	}
	return rows.Err()
}

func (engine *Engine) createOrAggregateAlert(ctx context.Context, rule Rule, event parser.NormalizedEvent) error {
	entityKey := getEntityKey(event)

	var condition struct {
		CooldownSeconds int `json:"cooldown_seconds"`
	}
	_ = json.Unmarshal(rule.Condition, &condition)

	cooldownWindow := condition.CooldownSeconds
	if cooldownWindow <= 0 {
		cooldownWindow = 180 // Default 3 minutes cooldown / aggregation window
	}

	transaction, err := engine.postgres.Begin(ctx)
	if err != nil {
		return err
	}
	defer transaction.Rollback(ctx)

	// Check for an existing open or acknowledged alert for (rule_id, entity_key) within cooldown window
	var existingAlertID int64
	var existingOccurrences int
	err = transaction.QueryRow(ctx, `
		SELECT alert_id, occurrences 
		FROM alerts 
		WHERE rule_id = $1 
		  AND entity_key = $2 
		  AND status IN ('open', 'acknowledged')
		  AND last_seen >= (now() - ($3 || ' seconds')::interval)
		ORDER BY last_seen DESC 
		LIMIT 1
	`, rule.ID, entityKey, cooldownWindow).Scan(&existingAlertID, &existingOccurrences)

	if err == nil && existingAlertID > 0 {
		// Aggregate into existing alert
		newOccurrences := existingOccurrences + 1
		summary := fmt.Sprintf("%s: %s (⚡ %dx aggregated)", rule.Name, event.Message, newOccurrences)

		_, err = transaction.Exec(ctx, `
			UPDATE alerts 
			SET occurrences = $1, last_seen = now(), summary = $2 
			WHERE alert_id = $3
		`, newOccurrences, summary, existingAlertID)
		if err != nil {
			return err
		}

		_, _ = transaction.Exec(ctx, `
			INSERT INTO alert_events (alert_id, event_id) 
			VALUES ($1, $2) 
			ON CONFLICT DO NOTHING
		`, existingAlertID, event.EventID)

		return transaction.Commit(ctx)
	}

	// Create new alert
	summary := rule.Name + ": " + event.Message
	var alertID int64
	err = transaction.QueryRow(ctx, `
		INSERT INTO alerts (rule_id, severity, summary, entity_key, occurrences, triggered_at, last_seen) 
		VALUES ($1, $2, $3, $4, 1, now(), now()) 
		RETURNING alert_id
	`, rule.ID, rule.Severity, summary, entityKey).Scan(&alertID)
	if err != nil {
		return err
	}

	if _, err := transaction.Exec(ctx, `
		INSERT INTO alert_events (alert_id, event_id) 
		VALUES ($1, $2) 
		ON CONFLICT DO NOTHING
	`, alertID, event.EventID); err != nil {
		return err
	}

	if err := transaction.Commit(ctx); err != nil {
		return err
	}

	// Send notification for new alert creation
	notifyText := fmt.Sprintf("[%s] %s\nHost: %s | Entity: %s\n%s", rule.Severity, rule.Name, event.Hostname, entityKey, event.Message)
	if err := engine.notifier.Send(ctx, notifyText); err != nil {
		fmt.Printf("telegram notification failed: %v\n", err)
	}

	return nil
}

func (engine *Engine) shouldAlert(ctx context.Context, rule Rule, event parser.NormalizedEvent) bool {
	var condition struct {
		Count           int `json:"count"`
		WindowSeconds   int `json:"window_seconds"`
		CooldownSeconds int `json:"cooldown_seconds"`
	}
	if len(rule.Condition) == 0 || json.Unmarshal(rule.Condition, &condition) != nil {
		return true
	}

	entityKey := getEntityKey(event)

	if condition.Count > 1 {
		key := fmt.Sprintf("siem:correlation:%d:%s", rule.ID, entityKey)
		count, err := engine.redis.Incr(ctx, key).Result()
		if err != nil {
			return false
		}
		window := time.Duration(condition.WindowSeconds) * time.Second
		if window <= 0 {
			window = time.Minute
		}
		if count == 1 {
			_ = engine.redis.Expire(ctx, key, window).Err()
		}
		if count < int64(condition.Count) {
			return false
		}
		_ = engine.redis.Del(ctx, key).Err()
	}

	return true
}

func getEntityKey(event parser.NormalizedEvent) string {
	if event.SrcIP != "" {
		return event.SrcIP
	}
	if event.Username != "" {
		return event.Username
	}
	if event.Hostname != "" {
		return event.Hostname
	}
	return "unknown"
}

type TelegramNotifier struct {
	token, chatID string
	client        *http.Client
}

func NewTelegramNotifier(token, chatID string) *TelegramNotifier {
	return &TelegramNotifier{token: token, chatID: chatID, client: &http.Client{Timeout: 10 * time.Second}}
}

func (notifier *TelegramNotifier) Send(ctx context.Context, text string) error {
	if notifier.token == "" || notifier.chatID == "" {
		return nil
	}
	payload, err := json.Marshal(map[string]string{"chat_id": notifier.chatID, "text": text})
	if err != nil {
		return err
	}
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.telegram.org/bot"+notifier.token+"/sendMessage", bytes.NewReader(payload))
		if err != nil {
			return err
		}
		request.Header.Set("Content-Type", "application/json")
		response, err := notifier.client.Do(request)
		if err == nil && response.StatusCode < 300 {
			response.Body.Close()
			return nil
		}
		if response != nil {
			response.Body.Close()
			lastErr = fmt.Errorf("Telegram API: %s", response.Status)
		} else {
			lastErr = err
		}
		if attempt < 2 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(1<<attempt) * 250 * time.Millisecond):
			}
		}
	}
	return lastErr
}

func fieldValue(event parser.NormalizedEvent, field string) string {
	switch field {
	case "message":
		return event.Message
	case "event_type":
		return event.EventType
	case "log_category":
		return event.LogCategory
	case "severity":
		return event.Severity
	case "src_ip":
		return event.SrcIP
	case "username":
		return event.Username
	case "hostname":
		return event.Hostname
	case "agent_id":
		return event.AgentID
	default:
		return event.Extra[field]
	}
}
