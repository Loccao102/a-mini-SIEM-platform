package correlation

import (
	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/parser"
	"testing"
	"time"
)

func fixture() []parser.NormalizedEvent {
	out := []parser.NormalizedEvent{}
	now := time.Now()
	for i, kind := range []string{"authentication_failure", "authentication_failure", "authentication_failure", "authentication_success", "privilege_escalation"} {
		out = append(out, parser.NormalizedEvent{EventID: string(rune('a' + i)), Hostname: "lab", Username: "alice", EventType: kind, EventTime: now.Add(time.Duration(i) * time.Second)})
	}
	return out
}
func TestCorrelationEvidenceAndIsolation(t *testing.T) {
	events := fixture()
	got := Match(events)
	if len(got) != 1 || len(got[0].IDs) != 5 {
		t.Fatal(got)
	}
	events[0], events[4] = events[4], events[0]
	if len(Match(events)) != 1 {
		t.Fatal("arrival order changed result")
	}
	events = fixture()
	events[3].Username = "bob"
	if len(Match(events)) != 0 {
		t.Fatal("cross-user chain")
	}
	events = fixture()
	events[2].Hostname = "other"
	if len(Match(events)) != 0 {
		t.Fatal("cross-host chain")
	}
	events = fixture()
	events[0].EventTime = events[0].EventTime.Add(-11 * time.Minute)
	if len(Match(events)) != 0 {
		t.Fatal("expired window")
	}
	events = fixture()
	events[1].EventID = events[0].EventID
	if len(Match(events)) != 0 {
		t.Fatal("duplicate counted toward threshold")
	}
}
