package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func eventSearchQuery(v url.Values) (map[string]any, int, int, error) {
	page, size := 1, 25
	var err error
	if v.Get("page") != "" {
		page, err = strconv.Atoi(v.Get("page"))
		if err != nil || page < 1 {
			return nil, 0, 0, fmt.Errorf("invalid page")
		}
	}
	if v.Get("page_size") != "" {
		size, err = strconv.Atoi(v.Get("page_size"))
		if err != nil || (size != 25 && size != 50 && size != 100) {
			return nil, 0, 0, fmt.Errorf("page_size must be 25, 50 or 100")
		}
	}
	if page > 10000/size {
		return nil, 0, 0, fmt.Errorf("pagination exceeds 10000 results; narrow filters")
	}
	clauses := []any{}
	if q := strings.TrimSpace(v.Get("q")); q != "" {
		if len(q) > 512 {
			return nil, 0, 0, fmt.Errorf("query too long")
		}
		clauses = append(clauses, map[string]any{"multi_match": map[string]any{"query": q, "fields": []string{"message", "hostname", "log_category", "event_type", "username"}}})
	}
	for _, field := range []string{"severity", "hostname", "log_category", "event_type", "src_ip", "username"} {
		if val := v.Get(field); val != "" {
			clauses = append(clauses, map[string]any{"term": map[string]any{field + ".keyword": val}})
		}
	}
	bounds := map[string]any{}
	var start, end time.Time
	for _, key := range []string{"from", "to"} {
		if val := v.Get(key); val != "" {
			ts, e := time.Parse(time.RFC3339Nano, val)
			if e != nil {
				return nil, 0, 0, fmt.Errorf("%s must be RFC3339", key)
			}
			if key == "from" {
				start = ts
				bounds["gte"] = ts.UTC().Format(time.RFC3339Nano)
			} else {
				end = ts
				bounds["lte"] = ts.UTC().Format(time.RFC3339Nano)
			}
		}
	}
	if !start.IsZero() && !end.IsZero() && start.After(end) {
		return nil, 0, 0, fmt.Errorf("from must not exceed to")
	}
	if len(bounds) > 0 {
		clauses = append(clauses, map[string]any{"range": map[string]any{"event_time": bounds}})
	}
	return map[string]any{"from": (page - 1) * size, "size": size, "track_total_hits": true, "sort": []any{map[string]any{"event_time": "desc"}, map[string]any{"event_id.keyword": map[string]any{"order": "asc", "unmapped_type": "keyword"}}}, "query": map[string]any{"bool": map[string]any{"must": clauses}}}, page, size, nil
}

func (h *Handler) createIngestKey(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		writeError(w, 400, fmt.Errorf("invalid asset ID"))
		return
	}
	ttl := 90 * 24 * time.Hour
	key, raw, err := h.apiKeyManager.GenerateKey(r.Context(), id, &ttl)
	if err != nil {
		writeError(w, 400, err)
		return
	}
	_, err = h.postgres.Exec(r.Context(), `INSERT INTO audit_logs(actor_user_id,action,entity_type,entity_id,details) VALUES($1,'api_key.created','api_key',$2,'{}')`, h.claims(r).UserID, key.KeyID)
	if err != nil {
		_ = h.apiKeyManager.RevokeKey(r.Context(), key.KeyID)
		writeError(w, 500, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 201, map[string]any{"api_key_id": key.KeyID, "api_key": raw, "expires_at": key.ExpiresAt})
}

func (h *Handler) detectionFindings(w http.ResponseWriter, r *http.Request) {
	rows, err := h.postgres.Query(r.Context(), `SELECT finding_id,alert_id,rule_version,techniques,evidence,created_at FROM detection_findings ORDER BY created_at DESC,finding_id LIMIT 100`)
	if err != nil {
		writeError(w, 500, err)
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, version string
		var alertID int64
		var techniques, evidence json.RawMessage
		var created time.Time
		if err = rows.Scan(&id, &alertID, &version, &techniques, &evidence, &created); err != nil {
			writeError(w, 500, err)
			return
		}
		out = append(out, map[string]any{"finding_id": id, "alert_id": alertID, "rule_version": version, "techniques": techniques, "evidence": evidence, "created_at": created})
	}
	if err = rows.Err(); err != nil {
		writeError(w, 500, err)
		return
	}
	writeJSON(w, 200, out)
}
