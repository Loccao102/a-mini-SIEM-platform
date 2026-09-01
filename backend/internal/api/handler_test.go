package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWithCORSSupportsFrontendPreflight(t *testing.T) {
	request := httptest.NewRequest(http.MethodOptions, "/api/v1/auth/login", nil)
	request.Header.Set("Origin", "http://localhost:3000")
	request.Header.Set("Access-Control-Request-Method", http.MethodPost)
	request.Header.Set("Access-Control-Request-Headers", "content-type")
	response := httptest.NewRecorder()

	withCORS(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		t.Fatal("preflight request reached the wrapped handler")
	})).ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("expected preflight status 204, got %d", response.Code)
	}
	if got := response.Header().Get("Access-Control-Allow-Headers"); got != "Content-Type, Authorization" {
		t.Fatalf("unexpected allowed headers: %q", got)
	}
	if got := response.Header().Get("Access-Control-Allow-Methods"); got != "GET, POST, PUT, PATCH, DELETE, OPTIONS" {
		t.Fatalf("unexpected allowed methods: %q", got)
	}
}

func TestResourceRoutesRejectUnsupportedMethods(t *testing.T) {
	tests := []struct {
		name   string
		handle func(http.ResponseWriter, *http.Request)
		method string
		path   string
	}{
		{name: "rules", handle: (&Handler{}).rulesRoute, method: http.MethodPatch, path: "/api/v1/rules"},
		{name: "alerts", handle: (&Handler{}).alertsRoute, method: http.MethodPost, path: "/api/v1/alerts/1"},
		{name: "users", handle: (&Handler{}).usersRoute, method: http.MethodPatch, path: "/api/v1/users"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, nil)
			response := httptest.NewRecorder()
			test.handle(response, request)
			if response.Code != http.StatusMethodNotAllowed {
				t.Fatalf("expected status 405, got %d", response.Code)
			}
		})
	}
}

func TestNormalizeFleetEnrollment(t *testing.T) {
	payload := fleetEnrollmentRequest{
		Hostname:    "web-02",
		SourceTypes: []string{"system", "docker"},
		Tags:        map[string]string{"env": "prod"},
	}
	result, err := normalizeFleetEnrollment(payload)
	if err != nil {
		t.Fatalf("normalizeFleetEnrollment returned error: %v", err)
	}
	if result.OSType != "linux" {
		t.Fatalf("expected default os_type linux, got %q", result.OSType)
	}
	if result.AgentID != "fleet-agent" {
		t.Fatalf("expected default agent_id fleet-agent, got %q", result.AgentID)
	}
	if len(result.SourceTypes) != 2 {
		t.Fatalf("expected 2 source types, got %d", len(result.SourceTypes))
	}
	if result.Tags["fleet.agent_id"] != "fleet-agent" {
		t.Fatalf("fleet agent tag not normalized: %#v", result.Tags)
	}
}
