package storage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBulkIndexEventsUsesEventIDsAndNDJSON(t *testing.T) {
	var requestBody string
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/normalized_events/_bulk" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		if got := request.Header.Get("Content-Type"); got != "application/x-ndjson" {
			t.Fatalf("unexpected content type: %s", got)
		}
		buffer := make([]byte, request.ContentLength)
		_, _ = request.Body.Read(buffer)
		requestBody = string(buffer)
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"errors":false,"items":[{"index":{"status":201}},{"index":{"status":201}}]}`))
	}))
	defer server.Close()

	client := NewElasticsearch(server.URL)
	err := client.BulkIndexEvents(context.Background(), []any{
		map[string]any{"event_id": "event-1", "message": "one"},
		map[string]any{"event_id": "event-2", "message": "two"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(requestBody, `"_index":"normalized_events"`) || !strings.Contains(requestBody, `"_id":"event-1"`) || !strings.Contains(requestBody, `"_id":"event-2"`) {
		t.Fatalf("bulk payload did not contain deterministic IDs: %s", requestBody)
	}
	if strings.Count(requestBody, "\n") != 4 {
		t.Fatalf("expected four NDJSON lines, got %d", strings.Count(requestBody, "\n"))
	}
}

func TestBulkIndexEventsReturnsItemErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"errors":true,"items":[{"index":{"status":400,"error":{"reason":"bad mapping"}}}]}`))
	}))
	defer server.Close()

	err := NewElasticsearch(server.URL).BulkIndexEvents(context.Background(), []any{map[string]any{"event_id": "event-1"}})
	if err == nil || !strings.Contains(err.Error(), "bad mapping") {
		t.Fatalf("expected bulk item error, got %v", err)
	}
}
