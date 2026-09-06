package storage

import (
	"testing"
)

func TestDailyRoutingStableAcrossTimezones(t *testing.T) {
	for _, ts := range []string{"2026-09-05T23:30:00Z", "2026-09-06T06:30:00+07:00"} {
		got, err := eventIndex(map[string]any{"event_time": ts})
		if err != nil || got != "siem-events-2026.09.05" {
			t.Fatalf("%s %v", got, err)
		}
	}
	if _, err := eventIndex(map[string]any{}); err == nil {
		t.Fatal("missing timestamp accepted")
	}
}
