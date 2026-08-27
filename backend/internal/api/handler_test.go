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
