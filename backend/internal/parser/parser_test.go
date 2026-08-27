package parser

import (
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestParseFailedSSHLogin(t *testing.T) {
	event := Parse(redis.XMessage{ID: "1-0", Values: map[string]any{"raw": "Failed password for root from 192.0.2.10 port 55222 ssh2", "source_type": "linux_sshd", "received_at": "2026-08-27T12:00:00Z", "hostname": "web-01"}})
	if event.EventType != "authentication_failure" || event.Severity != "medium" || event.Username != "root" || event.SrcIP != "192.0.2.10" || event.Extra["src_port"] != "55222" {
		t.Fatalf("unexpected event: %#v", event)
	}
	if !event.EventTime.Equal(time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected event time: %s", event.EventTime)
	}
}

func TestParseGenericLogPreservesRawMessage(t *testing.T) {
	event := Parse(redis.XMessage{ID: "2-0", Values: map[string]any{"raw": "hello", "source_type": "generic"}})
	if event.EventType != "log" || event.Message != "hello" || event.LogCategory != "generic" {
		t.Fatalf("unexpected generic event: %#v", event)
	}
}

func TestParseWindowsLogonFailure(t *testing.T) {
	event := Parse(redis.XMessage{ID: "3-0", Values: map[string]any{"raw": "An account failed to log on Account Name: alice Source Network Address: 192.0.2.20", "source_type": "windows_eventlog"}})
	if event.LogCategory != "windows_security" || event.Username != "alice" || event.SrcIP != "192.0.2.20" {
		t.Fatalf("unexpected Windows event: %#v", event)
	}
}

func TestParseNginxAccess(t *testing.T) {
	event := Parse(redis.XMessage{ID: "4-0", Values: map[string]any{"raw": "192.0.2.30 - - [27/Aug/2026:12:00:00 +0000] \"GET /admin HTTP/1.1\" 404 120", "source_type": "nginx"}})
	if event.EventType != "http_access" || event.SrcIP != "192.0.2.30" || event.Extra["status_code"] != "404" {
		t.Fatalf("unexpected nginx event: %#v", event)
	}
}
