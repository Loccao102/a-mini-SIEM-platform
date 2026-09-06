//go:build integration

package integration

import (
	"context"
	"fmt"
	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/correlation"
	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/parser"
	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/ruleengine"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

func phaseDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(env("INTEGRATION_POSTGRES_URL", "postgres://siem:siem_dev_password@localhost:5432/siem?sslmode=disable"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.MaxConns = 30
	p, e := pgxpool.NewWithConfig(context.Background(), cfg)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(p.Close)
	return p
}
func TestConcurrentReplayIsIdempotentAcrossEngineRestart(t *testing.T) {
	p := phaseDB(t)
	ctx := context.Background()
	var id int64
	pattern := fmt.Sprintf("replay-%d", time.Now().UnixNano())
	err := p.QueryRow(ctx, `INSERT INTO rules(name,regex_pattern,target_field,severity,category) VALUES($1,$1,'message','high','test') RETURNING rule_id`, pattern).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	engine, err := ruleengine.New(p, env("INTEGRATION_REDIS_URL", "redis://localhost:6379/0"), "", "")
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	ev := parser.NormalizedEvent{EventID: pattern, EventTime: time.Now(), Hostname: "replay-host", Message: pattern}
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- engine.Process(ctx, ev) }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	restarted, err := ruleengine.New(p, env("INTEGRATION_REDIS_URL", "redis://localhost:6379/0"), "", "")
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	if err = restarted.Process(ctx, ev); err != nil {
		t.Fatal(err)
	}
	var count, occ int
	if err = p.QueryRow(ctx, `SELECT count(*),COALESCE(sum(occurrences),0) FROM alerts WHERE rule_id=$1`, id).Scan(&count, &occ); err != nil || count != 1 || occ != 1 {
		t.Fatalf("count=%d occurrences=%d err=%v", count, occ, err)
	}
}
func TestCorrelationPersistsOutOfOrderAndReplay(t *testing.T) {
	p := phaseDB(t)
	ctx := context.Background()
	now := time.Now()
	host := fmt.Sprintf("chain-%d", now.UnixNano())
	events := []parser.NormalizedEvent{}
	for i, k := range []string{"authentication_failure", "authentication_failure", "authentication_failure", "authentication_success", "privilege_escalation"} {
		events = append(events, parser.NormalizedEvent{EventID: fmt.Sprintf("%s-%d", host, i), Hostname: host, Username: "alice", EventType: k, EventTime: now.Add(time.Duration(i-5) * time.Second)})
	}
	for _, i := range []int{4, 2, 0, 3, 1, 4, 0} {
		engine := &correlation.Engine{DB: p}
		if err := engine.Process(ctx, events[i]); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := p.QueryRow(ctx, `SELECT count(*) FROM detection_findings WHERE evidence->>'hostname'=$1`, host).Scan(&count); err != nil || count != 1 {
		t.Fatalf("findings=%d %v", count, err)
	}
}
func TestIngestRejectsSpoofedHostAndUnenrolledSource(t *testing.T) {
	key, _ := provisionIngestKey(t)
	for _, body := range []string{`{"message":"bad","hostname":"other-host","agent_id":"integration-agent","source_type":"integration_test"}`, `{"message":"bad","hostname":"integration-host","agent_id":"other-agent","source_type":"integration_test"}`, `{"message":"bad","hostname":"integration-host","agent_id":"integration-agent","source_type":"unknown"}`} {
		req, _ := http.NewRequest("POST", env("INTEGRATION_API_URL", "http://localhost:8080")+"/api/v1/ingest", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+key)
		res, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != 403 {
			t.Fatalf("expected403 got%s", res.Status)
		}
	}
}
func TestEnrollmentRequiresAuthentication(t *testing.T) {
	req, _ := http.NewRequest("POST", env("INTEGRATION_API_URL", "http://localhost:8080")+"/api/v1/fleet/agents", strings.NewReader(`{"hostname":"unauthorized","agent_id":"bad"}`))
	res, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 401 {
		t.Fatal(res.Status)
	}
}
