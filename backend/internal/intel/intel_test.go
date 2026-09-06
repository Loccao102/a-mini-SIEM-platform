package intel

import (
	"context"
	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/parser"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

type counting struct {
	count int
	feed  Feed
}

func (p *counting) Lookup(ctx context.Context, i Indicator) ([]Observation, error) {
	p.count++
	return p.feed.Lookup(ctx, i)
}
func TestFeedCacheExpiryAndProvenance(t *testing.T) {
	now := time.Now()
	p := &counting{feed: Feed{{Indicator: Indicator{"ip", "8.8.8.8"}, Source: "test-fixture", License: "CC0", UpdatedAt: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour), Malicious: true, Country: "Fixture"}}}
	e := New(p)
	for j := 0; j < 2; j++ {
		ev := parser.NormalizedEvent{SrcIP: "8.8.8.8", Raw: "unchanged", Severity: "info"}
		e.Enrich(context.Background(), &ev)
		if ev.Severity != "high" || ev.Raw != "unchanged" || ev.Extra["intel_evidence"] == "[]" {
			t.Fatal(ev)
		}
	}
	if p.count != 1 {
		t.Fatal(p.count)
	}
	e.cache = map[Indicator]cached{}
	p.feed[0].ExpiresAt = now.Add(-time.Second)
	ev := parser.NormalizedEvent{SrcIP: "8.8.8.8", Severity: "info"}
	e.Enrich(context.Background(), &ev)
	if ev.Severity != "info" {
		t.Fatal("expired evidence accepted")
	}
}
func TestPrivateIPv6AndQuota(t *testing.T) {
	p := &counting{}
	e := New(p)
	for _, ip := range []string{"172.25.1.1", "fc00::123", "::ffff:192.168.1.1", "::1"} {
		ev := parser.NormalizedEvent{SrcIP: ip}
		e.Enrich(context.Background(), &ev)
	}
	if p.count != 0 {
		t.Fatal("private addresses leaked")
	}
	e.MaxPerMinute = 1
	for _, ip := range []string{"8.8.8.8", "1.1.1.1"} {
		ev := parser.NormalizedEvent{SrcIP: ip}
		e.Enrich(context.Background(), &ev)
	}
	if p.count != 1 {
		t.Fatal("quota not enforced")
	}
}
func TestHTTPTimeoutDoesNotBlockIngest(t *testing.T) {
	var calls atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); <-r.Context().Done() }))
	defer s.Close()
	e := New(HTTPProvider{URL: s.URL})
	e.Timeout = 20 * time.Millisecond
	ev := parser.NormalizedEvent{SrcIP: "8.8.8.8", Raw: "original"}
	start := time.Now()
	e.Enrich(context.Background(), &ev)
	if time.Since(start) > time.Second || ev.Extra["intel_status"] != "unavailable" || ev.Raw != "original" {
		t.Fatal(ev)
	}
}
func TestExtractIndicators(t *testing.T) {
	ev := parser.NormalizedEvent{SrcIP: "2001:4860:4860::8888", Raw: "visit https://EXAMPLE.com/path and https://example.com/"}
	got := Extract(ev)
	if len(got) != 2 || got[1].Value != "example.com" {
		t.Fatal(got)
	}
}
