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

func TestParseSudoEscalation(t *testing.T) {
	event := Parse(redis.XMessage{ID: "2-0", Values: map[string]any{"raw": "sudo:  alice : TTY=pts/0 ; PWD=/home/alice ; USER=root ; COMMAND=/bin/bash -i", "source_type": "linux_audit"}})
	if event.EventType != "privilege_escalation" || event.Severity != "high" || event.Username != "alice" || event.Extra["command"] != "/bin/bash -i" {
		t.Fatalf("unexpected sudo escalation event: %#v", event)
	}
}

func TestParseWindowsAuditCleared(t *testing.T) {
	event := Parse(redis.XMessage{ID: "3-0", Values: map[string]any{"raw": "Event ID: 1102 The audit log was cleared", "source_type": "windows_eventlog"}})
	if event.EventType != "defense_evasion" || event.Severity != "critical" || event.LogCategory != "windows_security" {
		t.Fatalf("unexpected Windows audit log clear event: %#v", event)
	}
}

func TestParseWebSQLInjection(t *testing.T) {
	event := Parse(redis.XMessage{ID: "4-0", Values: map[string]any{"raw": "192.0.2.30 - - [27/Aug/2026:12:00:00 +0000] \"GET /api/user?id=1 UNION SELECT 1,2,3 HTTP/1.1\" 200 450", "source_type": "nginx"}})
	if event.EventType != "web_sqli" || event.Severity != "critical" || event.SrcIP != "192.0.2.30" {
		t.Fatalf("unexpected Web SQLi event: %#v", event)
	}
}

func TestParseWebPathTraversal(t *testing.T) {
	event := Parse(redis.XMessage{ID: "5-0", Values: map[string]any{"raw": "192.0.2.40 - - [27/Aug/2026:12:00:00 +0000] \"GET /download?file=../../../../etc/passwd HTTP/1.1\" 403 120", "source_type": "nginx"}})
	if event.EventType != "web_lfi" || event.Severity != "high" || event.SrcIP != "192.0.2.40" {
		t.Fatalf("unexpected Web LFI event: %#v", event)
	}
}

func TestFingerprintConsistency(t *testing.T) {
	now := time.Now()
	fp1 := Fingerprint("web-01", now, "Failed password for root from 1.2.3.4")
	fp2 := Fingerprint("web-01", now, "Failed password for root from 1.2.3.4")
	if fp1 == "" || fp1 != fp2 {
		t.Fatalf("fingerprint consistency check failed: %s vs %s", fp1, fp2)
	}
}
