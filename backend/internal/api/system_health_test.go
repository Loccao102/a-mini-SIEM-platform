package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/auth"
	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/health"
)

func TestPublicHealthResponseNeverIncludesInternalDetails(t *testing.T) {
	result := HealthzResponse{Status: "healthy", Uptime: "10s"}
	encoded, err := json.Marshal(result)
	if err != nil { t.Fatal(err) }
	if strings.Contains(string(encoded), "services") || strings.Contains(string(encoded), "checks") {
		t.Fatal("Public probe response must not include internal checks")
	}
	h := &Handler{}
	handler := h.handleServiceHealth(func(context.Context) health.ServiceHealth {
		return health.ServiceHealth{
			Status: health.StatusUnhealthy,
			Message: "password=secret postgres internal hostname",
			Checks: map[string]string{"postgres_url":"postgres://private:pass"},
		}
	})
	record := httptest.NewRecorder()
	handler(record, httptest.NewRequest(http.MethodGet, "/healthz/redis", nil))
	if record.Code != http.StatusServiceUnavailable {
		t.Fatalf("Expected 503 from unhealthy probe, got %d", record.Code)
	}
	body := record.Body.String()
	if strings.Contains(body, "secret") || strings.Contains(body, "postgres_url") || strings.Contains(body, "hostname") {
		t.Fatalf("Probe leaked private data: %s", body)
	}
	if !strings.Contains(body, "unhealthy") {
		t.Fatalf("Expected status-only response, got %s", body)
	}
}

func TestSystemDiagnosticsAndMetricsRequireAdmin(t *testing.T) {
	manager := auth.NewManager("unit-test-secret-with-enough-entropy-to-use")
	handler := (&Handler{auth: manager}).Routes()
	for _, path := range []string{"/api/v1/system/health", "/metrics"} {
		t.Run(path,func(t *testing.T){
			request := httptest.NewRequest(http.MethodGet, path, nil)
			record := httptest.NewRecorder()
			handler.ServeHTTP(record,request)
			if record.Code!=http.StatusUnauthorized {
				t.Fatalf("Unidentified request got HTTP %d", record.Code)
			}

			token, err := manager.Token(2, "viewer@example.org", "viewer")
			if err != nil { t.Fatal(err) }
			withViewer:=httptest.NewRequest(http.MethodGet,path,nil)
			withViewer.Header.Set("Authorization","Bearer "+token)
			denied:=httptest.NewRecorder()
			handler.ServeHTTP(denied,withViewer)
			if denied.Code!=http.StatusForbidden {
				t.Fatalf("Viewer request got HTTP %d, expected 403", denied.Code)
			}
		})
	}
}
