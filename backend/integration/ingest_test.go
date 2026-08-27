//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestHTTPIngestPublishesToRedisStream(t *testing.T) {
	apiURL := env("INTEGRATION_API_URL", "http://localhost:8080")
	redisURL := env("INTEGRATION_REDIS_URL", "redis://localhost:6379/0")
	stream := env("REDIS_STREAM", "siem:raw-logs")
	redisOptions, err := redis.ParseURL(redisURL)
	if err != nil {
		t.Fatal(err)
	}
	redisClient := redis.NewClient(redisOptions)
	defer redisClient.Close()
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		t.Skipf("Redis is not available: %v", err)
	}

	raw := fmt.Sprintf("integration failed login %d", time.Now().UnixNano())
	payload := map[string]any{
		"event":       map[string]string{"original": raw},
		"source_type": "integration_test",
		"host":        map[string]string{"name": "integration-host"},
		"agent":       map[string]string{"id": "integration-agent"},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPost, apiURL+"/api/v1/ingest", strings.NewReader(string(body)))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := (&http.Client{Timeout: 10 * time.Second}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202 Accepted, got %s", response.Status)
	}

	var result struct {
		StreamID string `json:"stream_id"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.StreamID == "" {
		t.Fatal("ingest response did not contain stream_id")
	}
	entries, err := redisClient.XRange(context.Background(), stream, result.StreamID, result.StreamID).Result()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected one stream entry for %s, got %d", result.StreamID, len(entries))
	}
	if entries[0].Values["raw"] != raw || entries[0].Values["source_type"] != "integration_test" || entries[0].Values["hostname"] != "integration-host" {
		t.Fatalf("unexpected stream payload: %#v", entries[0].Values)
	}
}

func TestAuthenticationAndAdminRBAC(t *testing.T) {
	apiURL := env("INTEGRATION_API_URL", "http://localhost:8080")
	client := &http.Client{Timeout: 10 * time.Second}

	protectedRequest, err := http.NewRequest(http.MethodGet, apiURL+"/api/v1/users", nil)
	if err != nil {
		t.Fatal(err)
	}
	protectedResponse, err := client.Do(protectedRequest)
	if err != nil {
		t.Fatal(err)
	}
	protectedResponse.Body.Close()
	if protectedResponse.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected unauthenticated users request to return 401, got %s", protectedResponse.Status)
	}

	credentials := map[string]string{
		"email":    env("INTEGRATION_ADMIN_EMAIL", "admin@example.com"),
		"password": env("INTEGRATION_ADMIN_PASSWORD", "change-me-now"),
	}
	body, err := json.Marshal(credentials)
	if err != nil {
		t.Fatal(err)
	}
	loginRequest, err := http.NewRequest(http.MethodPost, apiURL+"/api/v1/auth/login", strings.NewReader(string(body)))
	if err != nil {
		t.Fatal(err)
	}
	loginRequest.Header.Set("Content-Type", "application/json")
	loginResponse, err := client.Do(loginRequest)
	if err != nil {
		t.Fatal(err)
	}
	defer loginResponse.Body.Close()
	if loginResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected admin login to return 200, got %s", loginResponse.Status)
	}
	var loginResult struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(loginResponse.Body).Decode(&loginResult); err != nil {
		t.Fatal(err)
	}
	if loginResult.Token == "" {
		t.Fatal("login response did not contain a token")
	}

	usersRequest, err := http.NewRequest(http.MethodGet, apiURL+"/api/v1/users", nil)
	if err != nil {
		t.Fatal(err)
	}
	usersRequest.Header.Set("Authorization", "Bearer "+loginResult.Token)
	usersResponse, err := client.Do(usersRequest)
	if err != nil {
		t.Fatal(err)
	}
	defer usersResponse.Body.Close()
	if usersResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected admin users request to return 200, got %s", usersResponse.Status)
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
