package correlation

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/parser"
	"github.com/jackc/pgx/v5/pgxpool"
)

const Version = "ssh-chain-v1"

var Techniques = []string{"T1110", "T1078", "T1548.003"}

type Engine struct {
	DB  *pgxpool.Pool
	Now func() time.Time
}
type Chain struct {
	IDs      []string  `json:"event_ids"`
	Hostname string    `json:"hostname"`
	Username string    `json:"username"`
	End      time.Time `json:"end"`
}

// Match uses event time, not arrival order, and never joins different entities.
func Match(events []parser.NormalizedEvent) []Chain {
	sorted := append([]parser.NormalizedEvent(nil), events...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].EventTime.Equal(sorted[j].EventTime) {
			return sorted[i].EventID < sorted[j].EventID
		}
		return sorted[i].EventTime.Before(sorted[j].EventTime)
	})
	out := []Chain{}
	for _, end := range sorted {
		if end.EventType != "privilege_escalation" || end.Hostname == "" || end.Username == "" {
			continue
		}
		var failures []string
		seen := map[string]bool{}
		var success string
		for _, ev := range sorted {
			if ev.Hostname != end.Hostname || ev.Username != end.Username || ev.EventTime.Before(end.EventTime.Add(-10*time.Minute)) || !ev.EventTime.Before(end.EventTime) || seen[ev.EventID] {
				continue
			}
			seen[ev.EventID] = true
			if ev.EventType == "authentication_failure" && success == "" {
				failures = append(failures, ev.EventID)
			}
			if ev.EventType == "authentication_success" && len(failures) >= 3 {
				success = ev.EventID
			}
		}
		if success != "" {
			ids := append(append([]string{}, failures...), success, end.EventID)
			out = append(out, Chain{ids, end.Hostname, end.Username, end.EventTime})
		}
	}
	return out
}
func (e *Engine) Process(ctx context.Context, event parser.NormalizedEvent) error {
	if event.EventID == "" || event.Hostname == "" || event.Username == "" {
		return nil
	}
	switch event.EventType {
	case "authentication_failure", "authentication_success", "privilege_escalation":
	default:
		return nil
	}
	now := time.Now()
	if e.Now != nil {
		now = e.Now()
	}
	// Explicit lateness bound prevents unbounded retained correlation state.
	if event.EventTime.Before(now.Add(-15*time.Minute)) || event.EventTime.After(now.Add(time.Minute)) {
		return nil
	}
	tx, err := e.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, event.Hostname+"\x1f"+event.Username); err != nil {
		return err
	}
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO detection_events(event_id,hostname,username,event_time,event_type,evidence) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING`, event.EventID, event.Hostname, event.Username, event.EventTime, event.EventType, data); err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `SELECT evidence FROM detection_events WHERE hostname=$1 AND username=$2 AND event_time>=$3 ORDER BY event_time,event_id LIMIT 5001`, event.Hostname, event.Username, now.Add(-25*time.Minute))
	if err != nil {
		return err
	}
	events := []parser.NormalizedEvent{}
	for rows.Next() {
		var b []byte
		var ev parser.NormalizedEvent
		if err = rows.Scan(&b); err != nil {
			rows.Close()
			return err
		}
		if err = json.Unmarshal(b, &ev); err != nil {
			rows.Close()
			return err
		}
		events = append(events, ev)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if len(events) > 5000 {
		return fmt.Errorf("correlation entity window exceeds 5000 events")
	}
	for _, chain := range Match(events) {
		// One finding per terminal event, regardless of replay or late earlier evidence.
		sum := sha256.Sum256([]byte(Version + ":" + chain.IDs[len(chain.IDs)-1]))
		id := fmt.Sprintf("%x", sum)
		var exists bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM detection_findings WHERE finding_id=$1)`, id).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}
		var ruleID, alertID int64
		if err = tx.QueryRow(ctx, `SELECT rule_id FROM rules WHERE name='SSH attack chain v1' ORDER BY rule_id LIMIT 1`).Scan(&ruleID); err != nil {
			return err
		}
		if err = tx.QueryRow(ctx, `INSERT INTO alerts(rule_id,severity,summary,entity_key) VALUES($1,'critical',$2,$3) RETURNING alert_id`, ruleID, "SSH failures → success → privilege escalation", event.Hostname+":"+event.Username).Scan(&alertID); err != nil {
			return err
		}
		proof, _ := json.Marshal(chain)
		tech, _ := json.Marshal(Techniques)
		if _, err = tx.Exec(ctx, `INSERT INTO detection_findings(finding_id,alert_id,rule_version,techniques,evidence) VALUES($1,$2,$3,$4,$5)`, id, alertID, Version, tech, proof); err != nil {
			return err
		}
		for _, eventID := range chain.IDs {
			if _, err = tx.Exec(ctx, `INSERT INTO alert_events(alert_id,event_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, alertID, eventID); err != nil {
				return err
			}
		}
	}
	if _, err = tx.Exec(ctx, `DELETE FROM detection_events WHERE event_time<$1`, now.Add(-25*time.Minute)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
