package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ConfigureLifecycle preserves normalized_events as a read-only legacy source
// for this writer. Daily event-time routing keeps replay IDs in the same index.
// Call once at startup, before serving requests.
func (e *Elasticsearch) ConfigureLifecycle(ctx context.Context, days int) error {
	if days < 1 || days > 3650 {
		return fmt.Errorf("retention days must be 1..3650")
	}
	policy := map[string]any{"policy": map[string]any{"phases": map[string]any{"hot": map[string]any{"actions": map[string]any{}}, "delete": map[string]any{"min_age": fmt.Sprintf("%dd", days), "actions": map[string]any{"delete": map[string]any{}}}}}}
	if err := e.putConfig(ctx, "/_ilm/policy/siem-events-retention", policy); err != nil {
		return err
	}
	template := map[string]any{
		"index_patterns": []string{"siem-events-*"},
		"priority":       200,
		"template": map[string]any{
			"settings": map[string]any{
				"index.lifecycle.name":   "siem-events-retention",
				"number_of_shards":       1,
				"number_of_replicas":     0,
				"index.refresh_interval": "5s",
				"index.codec":            "best_compression",
			},
			"mappings": map[string]any{
				"properties": map[string]any{
					"event_time":   map[string]any{"type": "date"},
					"message":      map[string]any{"type": "text"},
					"extra_fields": map[string]any{"type": "object"},
				},
			},
		},
	}
	if err := e.putConfig(ctx, "/_index_template/siem-events", template); err != nil {
		return err
	}
	e.daily = true
	return nil
}
func (e *Elasticsearch) putConfig(ctx context.Context, path string, body any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, e.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := e.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return fmt.Errorf("configure %s: %s", path, res.Status)
	}
	return nil
}
func (e *Elasticsearch) readIndex() string {
	if e.daily {
		return "normalized_events,siem-events-*"
	}
	return "normalized_events"
}

// UseDailyIndices selects the combined read target without changing cluster configuration.
func (e *Elasticsearch) UseDailyIndices() { e.daily = true }
func eventIndex(fields map[string]any) (string, error) {
	raw, ok := fields["event_time"].(string)
	if !ok {
		return "", fmt.Errorf("event_time is required for retention routing")
	}
	ts, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return "", err
	}
	return "siem-events-" + ts.UTC().Format("2006.01.02"), nil
}
