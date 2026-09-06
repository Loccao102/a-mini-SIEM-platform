package soar

import (
	"testing"
)

func TestAllowlistProtection(t *testing.T) {
	engine := NewEngine(nil, []string{"my-trusted-host", "10.0.0.1"})

	testCases := []struct {
		target   string
		expected bool
	}{
		{"127.0.0.1", true},
		{"::1", true},
		{"localhost", true},
		{"ADMIN", true},
		{"root", true},
		{"my-trusted-host", true},
		{"10.0.0.1", true},
		{"", true}, // Empty target protected
		{"185.220.101.5", false},
		{"malicious-user", false},
		{"203.0.113.195", false},
	}

	for _, tc := range testCases {
		got := engine.IsAllowlisted(tc.target)
		if got != tc.expected {
			t.Errorf("IsAllowlisted(%q) = %v; want %v", tc.target, got, tc.expected)
		}
	}
}
