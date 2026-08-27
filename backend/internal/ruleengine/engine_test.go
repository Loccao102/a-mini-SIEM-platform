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
