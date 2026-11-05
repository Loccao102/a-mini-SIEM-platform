package health

import "testing"

func TestOverallFromChecks(t *testing.T) {
	tests := []struct {
		name string
		checks map[string]ServiceHealth
		want Status
	}{
		{"empty fails safe", nil, StatusUnhealthy},
		{"all healthy", map[string]ServiceHealth{
			"postgres": {Status: StatusHealthy},
			"redis": {Status: StatusHealthy},
		}, StatusHealthy},
		{"degraded when no outage", map[string]ServiceHealth{
			"postgres": {Status: StatusHealthy},
			"queues": {Status: StatusDegraded},
		}, StatusDegraded},
		{"unhealthy wins", map[string]ServiceHealth{
			"postgres": {Status: StatusDegraded},
			"redis": {Status: StatusUnhealthy},
		}, StatusUnhealthy},
		{"unknown status fails closed", map[string]ServiceHealth{
			"broken": {Status: Status("unknown")},
		}, StatusUnhealthy},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := OverallFromChecks(test.checks); got != test.want {
				t.Errorf("OverallFromChecks = %q, want %q", got, test.want)
			}
		})
	}
}
