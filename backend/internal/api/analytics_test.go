package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Loccao102/a-mini-SIEM-platform/backend/internal/storage"
)

func TestAnalyticsUsesElasticsearchAggregations(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/normalized_events/_search" {
			t.Fatalf("unexpected search path: %s", request.URL.Path)
		}
		var query map[string]any
		if err := json.NewDecoder(request.Body).Decode(&query); err != nil {
			t.Fatal(err)
		}
		if _, ok := query["aggs"]; !ok {
			t.Fatal("analytics query did not request aggregations")
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{
			"hits":{"total":{"value":2}},
			"aggregations":{
				"events_by_severity":{"buckets":[{"key":"critical","doc_count":2}]},
				"events_by_category":{"buckets":[{"key":"nginx","doc_count":2}]},
				"top_attacking_ips":{"buckets":[{"key":"203.0.113.10","doc_count":2,"sample":{"hits":{"hits":[{"_source":{"extra_fields":{"country":"Testland","country_code":"TT","reputation_score":"77"}}}]}}}]},
				"top_targeted_users":{"buckets":[{"key":"admin","doc_count":2}]},
				"timeline_last_24h":{"buckets":{"buckets":[{"key":1,"key_as_string":"2026-08-28T00:00:00.000Z","doc_count":2}]}}
			}
		}`))
	}))
	defer server.Close()

	handler := &Handler{elastic: storage.NewElasticsearch(server.URL)}
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/analytics", nil)
	response := httptest.NewRecorder()
	handler.analytics(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	var result struct {
		TotalEvents      int64            `json:"total_events"`
		EventsBySeverity map[string]int64 `json:"events_by_severity"`
		TopAttackingIPs  []map[string]any `json:"top_attacking_ips"`
		TopTargetedUsers []map[string]any `json:"top_targeted_users"`
		Timeline         []map[string]any `json:"timeline"`
	}
	if err := json.NewDecoder(strings.NewReader(response.Body.String())).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.TotalEvents != 2 || result.EventsBySeverity["critical"] != 2 {
		t.Fatalf("unexpected live analytics totals: %#v", result)
	}
	if len(result.TopAttackingIPs) != 1 || result.TopAttackingIPs[0]["ip"] != "203.0.113.10" {
		t.Fatalf("unexpected attacker aggregation: %#v", result.TopAttackingIPs)
	}
	if len(result.TopTargetedUsers) != 1 || len(result.Timeline) != 1 {
		t.Fatalf("unexpected remaining aggregations: %#v", result)
	}
}
