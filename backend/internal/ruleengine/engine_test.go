package ruleengine

import (
	"testing"

	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/parser"
)

func TestFieldValue(t *testing.T) {
	event := parser.NormalizedEvent{Message: "failed", SrcIP: "192.0.2.10", Extra: map[string]string{"src_port": "22"}}
	if fieldValue(event, "src_ip") != "192.0.2.10" || fieldValue(event, "src_port") != "22" || fieldValue(event, "missing") != "" {
		t.Fatal("event field lookup failed")
	}
}

func TestGetEntityKey(t *testing.T) {
	eventWithIP := parser.NormalizedEvent{SrcIP: "192.168.1.100", Username: "alice", Hostname: "host-1"}
	if getEntityKey(eventWithIP) != "192.168.1.100" {
		t.Fatalf("expected IP as entity key, got %s", getEntityKey(eventWithIP))
	}

	eventWithUser := parser.NormalizedEvent{Username: "bob", Hostname: "host-2"}
	if getEntityKey(eventWithUser) != "bob" {
		t.Fatalf("expected Username as entity key, got %s", getEntityKey(eventWithUser))
	}

	eventWithHost := parser.NormalizedEvent{Hostname: "host-3"}
	if getEntityKey(eventWithHost) != "host-3" {
		t.Fatalf("expected Hostname as entity key, got %s", getEntityKey(eventWithHost))
	}
}
