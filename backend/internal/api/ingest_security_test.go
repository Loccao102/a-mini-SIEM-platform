package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/health"
	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/ratelimit"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestIngestAuthenticatesBeforeRateLimiting(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { client.Close() })
	handler := &Handler{
		rateLimiter:   ratelimit.New(client, 1, time.Minute),
		healthChecker: health.New(nil, client),
	}
	routes := handler.Routes()
	for _, authorization := range []string{"", "Basic invalid", "Bearer "} {
		t.Run("authorization="+authorization, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/v1/ingest", strings.NewReader(`{"message":"test"}`))
			request.Header.Set("Authorization", authorization)
			response := httptest.NewRecorder()
			routes.ServeHTTP(response, request)
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401 before rate limiting, got %d: %s", response.Code, response.Body.String())
			}
			if response.Header().Get("X-RateLimit-Limit") != "" || len(server.Keys()) != 0 {
				t.Fatal("unauthenticated request consumed rate-limit state")
			}
		})
	}
}
