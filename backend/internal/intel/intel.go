// Package intel enriches events from explicit, attributable data sources.
package intel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/parser"
)

type Indicator struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}
type Observation struct {
	Indicator
	Source      string    `json:"source"`
	License     string    `json:"license"`
	UpdatedAt   time.Time `json:"updated_at"`
	ExpiresAt   time.Time `json:"expires_at"`
	Malicious   bool      `json:"malicious"`
	Country     string    `json:"country,omitempty"`
	CountryCode string    `json:"country_code,omitempty"`
	City        string    `json:"city,omitempty"`
}
type Provider interface {
	Lookup(context.Context, Indicator) ([]Observation, error)
}
type Feed []Observation

func LoadFeed(path string) (Feed, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var feed Feed
	err = json.NewDecoder(io.LimitReader(f, 8<<20)).Decode(&feed)
	return feed, err
}
func (f Feed) Lookup(ctx context.Context, i Indicator) ([]Observation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out := []Observation{}
	for _, o := range f {
		if o.Type != i.Type {
			continue
		}
		match := strings.EqualFold(o.Value, i.Value)
		if i.Type == "ip" {
			p, e := netip.ParsePrefix(o.Value)
			a, ae := netip.ParseAddr(i.Value)
			if e == nil && ae == nil {
				match = p.Contains(a)
			}
		}
		if match {
			out = append(out, o)
		}
	}
	return out, nil
}

type HTTPProvider struct {
	URL    string
	Token  string
	Client *http.Client
}

func (p HTTPProvider) Lookup(ctx context.Context, i Indicator) ([]Observation, error) {
	endpoint, err := url.Parse(p.URL)
	if err != nil {
		return nil, err
	}
	q := endpoint.Query()
	q.Set("type", i.Type)
	q.Set("value", i.Value)
	endpoint.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint.String(), nil)
	if err != nil {
		return nil, err
	}
	if p.Token != "" {
		req.Header.Set("Authorization", "Bearer "+p.Token)
	}
	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Second}
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("intel provider returned %d", res.StatusCode)
	}
	var out []Observation
	err = json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&out)
	return out, err
}

type cached struct {
	observations []Observation
	until        time.Time
	status       string
}
type Enricher struct {
	Provider     Provider
	Timeout      time.Duration
	TTL          time.Duration
	MaxPerMinute int
	mu           sync.Mutex
	cache        map[Indicator]cached
	window       time.Time
	used         int
}

func New(p Provider) *Enricher {
	return &Enricher{Provider: p, Timeout: 500 * time.Millisecond, TTL: 5 * time.Minute, MaxPerMinute: 120, cache: map[Indicator]cached{}}
}

var urlPattern = regexp.MustCompile(`https?://[^\s<>"']+`)
var hashPattern = regexp.MustCompile(`\b[[:xdigit:]]{64}\b`)

func Extract(event parser.NormalizedEvent) []Indicator {
	out := []Indicator{}
	seen := map[Indicator]bool{}
	add := func(i Indicator) {
		if len(out) < 32 && !seen[i] {
			seen[i] = true
			out = append(out, i)
		}
	}
	if a, e := netip.ParseAddr(event.SrcIP); e == nil {
		add(Indicator{"ip", a.Unmap().String()})
	}
	for _, u := range urlPattern.FindAllString(event.Raw, 32) {
		p, e := url.Parse(u)
		if e != nil || p.Hostname() == "" {
			continue
		}
		host := strings.ToLower(strings.TrimSuffix(p.Hostname(), "."))
		if a, e := netip.ParseAddr(host); e == nil {
			add(Indicator{"ip", a.Unmap().String()})
		} else {
			add(Indicator{"domain", host})
		}
	}
	for _, h := range hashPattern.FindAllString(event.Raw, 32) {
		add(Indicator{"sha256", strings.ToLower(h)})
	}
	return out
}
func (e *Enricher) lookup(ctx context.Context, i Indicator) ([]Observation, string) {
	if i.Type == "ip" {
		a, err := netip.ParseAddr(i.Value)
		if err != nil || a.IsPrivate() || a.IsLoopback() || a.IsLinkLocalUnicast() || !a.IsGlobalUnicast() {
			return nil, "non_public"
		}
	}
	if e.Provider == nil {
		return nil, "unconfigured"
	}
	now := time.Now()
	e.mu.Lock()
	if c, ok := e.cache[i]; ok && now.Before(c.until) {
		e.mu.Unlock()
		return c.observations, c.status
	}
	if now.Sub(e.window) >= time.Minute {
		e.window = now
		e.used = 0
	}
	if e.used >= e.MaxPerMinute {
		e.mu.Unlock()
		return nil, "quota_exceeded"
	}
	e.used++
	e.mu.Unlock()
	bounded, cancel := context.WithTimeout(ctx, e.Timeout)
	defer cancel()
	records, err := e.Provider.Lookup(bounded, i)
	status := "ok"
	ttl := e.TTL
	valid := []Observation{}
	if err != nil {
		status = "unavailable"
		ttl = 5 * time.Second
	} else {
		for _, o := range records {
			if o.Source == "" || o.License == "" || o.UpdatedAt.IsZero() || o.UpdatedAt.After(now) || !o.ExpiresAt.After(now) {
				continue
			}
			matched, _ := Feed{o}.Lookup(ctx, i)
			if len(matched) == 0 {
				continue
			}
			valid = append(valid, o)
			if remaining := o.ExpiresAt.Sub(now); remaining < ttl {
				ttl = remaining
			}
		}
	}
	e.mu.Lock()
	if len(e.cache) >= 4096 {
		e.cache = map[Indicator]cached{}
	}
	e.cache[i] = cached{valid, now.Add(ttl), status}
	e.mu.Unlock()
	return valid, status
}

// Enrich never returns a provider failure to the ingestion pipeline.
func (e *Enricher) Enrich(ctx context.Context, event *parser.NormalizedEvent) {
	if event.Extra == nil {
		event.Extra = map[string]string{}
	}
	indicators := Extract(*event)
	encoded, _ := json.Marshal(indicators)
	event.Extra["iocs"] = string(encoded)
	all := []Observation{}
	status := "ok"
	// One event has a fixed total budget, even if it contains many indicators.
	bounded, cancel := context.WithTimeout(ctx, e.Timeout)
	defer cancel()
	for _, i := range indicators {
		records, s := e.lookup(bounded, i)
		if s != "ok" {
			status = s
		}
		all = append(all, records...)
	}
	for _, o := range all {
		if o.Country != "" {
			event.Extra["country"] = o.Country
			event.Extra["country_code"] = o.CountryCode
			event.Extra["city"] = o.City
		}
		if o.Malicious {
			event.Extra["is_malicious"] = "true"
			event.Extra["severity_reason"] = "IOC matched: " + o.Source
			if event.Severity != "critical" {
				event.Severity = "high"
			}
		}
	}
	encoded, _ = json.Marshal(all)
	event.Extra["intel_evidence"] = string(encoded)
	event.Extra["intel_status"] = status
}
