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

	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/apikey"
	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/storage"
	"github.com/jackc/pgx/v5/pgxpool"
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
		t.Fatalf("Redis is not available: %v", err)
	}

	rawKey, _ := provisionIngestKey(t)
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
	request.Header.Set("Authorization", "Bearer "+rawKey)
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
	waitForIndexedEvent(t, raw)
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

// provisionIngestKey uses only the isolated integration database. Keys are
// generated through the production manager, never a hand-built database hash.
func provisionIngestKey(t *testing.T) (string, func()) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, env("INTEGRATION_POSTGRES_URL", "postgres://siem:siem_dev_password@localhost:5432/siem?sslmode=disable"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	var assetID int64
	err = pool.QueryRow(ctx, `INSERT INTO assets (hostname, os_type, criticality) VALUES ('integration-host', 'linux', 'low') ON CONFLICT (hostname) DO UPDATE SET os_type=EXCLUDED.os_type RETURNING asset_id`).Scan(&assetID)
	if err != nil {
		t.Fatal(err)
	}
	manager := apikey.New(pool)
	key, raw, err := manager.GenerateKey(ctx, assetID, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if _, err := pool.Exec(cleanupCtx, `DELETE FROM api_keys WHERE api_key_id=$1`, key.KeyID); err != nil {
			t.Errorf("cleanup key: %v", err)
		}
	})
	return raw, func() {
		ctx, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if err := manager.RevokeKey(ctx, key.KeyID); err != nil {
			t.Fatal(err)
		}
	}
}

func TestIngestRejectsMissingInvalidAndRevokedKeys(t *testing.T) {
	rawKey, revoke := provisionIngestKey(t)
	revoke()
	for _, key := range []string{"", "sk_invalid_key", rawKey} {
		request, err := http.NewRequest(http.MethodPost, env("INTEGRATION_API_URL", "http://localhost:8080")+"/api/v1/ingest", strings.NewReader(`{"message":"must not ingest","hostname":"integration-host"}`))
		if err != nil {
			t.Fatal(err)
		}
		if key != "" {
			request.Header.Set("Authorization", "Bearer "+key)
		}
		response, err := (&http.Client{Timeout: 10 * time.Second}).Do(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %s", response.Status)
		}
	}
}

func waitForIndexedEvent(t *testing.T, raw string) {
	t.Helper()
	elastic := storage.NewElasticsearch(env("INTEGRATION_ELASTICSEARCH_URL", "http://localhost:9200"))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for {
		result, err := elastic.Search(ctx, map[string]any{"query": map[string]any{"match_phrase": map[string]any{"raw": raw}}})
		if err != nil {
			t.Fatalf("search indexed event: %v", err)
		}
		var payload struct {
			Hits struct {
				Hits []struct {
					Source struct {
						Raw       string `json:"raw"`
						Hostname  string `json:"hostname"`
						EventType string `json:"event_type"`
					} `json:"_source"`
				} `json:"hits"`
			} `json:"hits"`
		}
		if err := json.Unmarshal(result, &payload); err != nil {
			t.Fatal(err)
		}
		for _, hit := range payload.Hits.Hits {
			if hit.Source.Raw == raw && hit.Source.Hostname == "integration-host" && hit.Source.EventType != "" {
				return
			}
		}
		select {
		case <-ctx.Done():
			t.Fatal("authenticated event was not normalized and indexed within 30 seconds")
		case <-time.After(250 * time.Millisecond):
		}
	}
}
