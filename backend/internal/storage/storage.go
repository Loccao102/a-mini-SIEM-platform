package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Clients struct {
	Postgres *pgxpool.Pool
	Elastic  *Elasticsearch
}

func OpenPostgres(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

type Elasticsearch struct {
	baseURL string
	client  *http.Client
}

func (elastic *Elasticsearch) Count(ctx context.Context) (int64, error) {
	result, err := elastic.Search(ctx, map[string]any{"size": 0, "track_total_hits": true, "query": map[string]any{"match_all": map[string]any{}}})
	if err != nil {
		return 0, err
	}
	var payload struct {
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
		} `json:"hits"`
	}
	if err := json.Unmarshal(result, &payload); err != nil {
		return 0, err
	}
	return payload.Hits.Total.Value, nil
}

func (elastic *Elasticsearch) Search(ctx context.Context, query map[string]any) (json.RawMessage, error) {
	payload, err := json.Marshal(query)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, elastic.baseURL+"/normalized_events/_search", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := elastic.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return nil, fmt.Errorf("search Elasticsearch events: %s", response.Status)
	}
	var result json.RawMessage
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

func NewElasticsearch(baseURL string) *Elasticsearch {
	return &Elasticsearch{baseURL: strings.TrimRight(baseURL, "/"), client: &http.Client{Timeout: 15 * time.Second}}
}

func (elastic *Elasticsearch) ClusterHealth(ctx context.Context) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, elastic.baseURL+"/_cluster/health", nil)
	if err != nil {
		return err
	}
	response, err := elastic.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return fmt.Errorf("cluster health: %s", response.Status)
	}
	var payload struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return err
	}
	if payload.Status == "red" || payload.Status == "" {
		return fmt.Errorf("cluster status is %q", payload.Status)
	}
	return nil
}

func (elastic *Elasticsearch) EnsureIndex(ctx context.Context) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodHead, elastic.baseURL+"/normalized_events", nil)
	if err != nil {
		return err
	}
	response, err := elastic.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusOK {
		return nil
	}
	if response.StatusCode != http.StatusNotFound {
		return fmt.Errorf("check Elasticsearch index: %s", response.Status)
	}
	payload := []byte(`{"mappings":{"properties":{"event_time":{"type":"date"},"message":{"type":"text"},"extra_fields":{"type":"object"}}}}`)
	request, err = http.NewRequestWithContext(ctx, http.MethodPut, elastic.baseURL+"/normalized_events", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err = elastic.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return fmt.Errorf("create Elasticsearch index: %s", response.Status)
	}
	return nil
}

func (elastic *Elasticsearch) IndexEvent(ctx context.Context, event any) error {
	return elastic.BulkIndexEvents(ctx, []any{event})
}

func (elastic *Elasticsearch) BulkIndexEvents(ctx context.Context, events []any) error {
	if len(events) == 0 {
		return nil
	}
	var payload bytes.Buffer
	for _, event := range events {
		encoded, err := json.Marshal(event)
		if err != nil {
			return err
		}
		metadata := map[string]any{"index": map[string]any{"_index": "normalized_events"}}
		var fields map[string]any
		if err := json.Unmarshal(encoded, &fields); err == nil {
			if eventID, ok := fields["event_id"].(string); ok && eventID != "" {
				metadata["index"].(map[string]any)["_id"] = eventID
			}
		}
		metadataJSON, err := json.Marshal(metadata)
		if err != nil {
			return err
		}
		payload.Write(metadataJSON)
		payload.WriteByte('\n')
		payload.Write(encoded)
		payload.WriteByte('\n')
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, elastic.baseURL+"/normalized_events/_bulk", &payload)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/x-ndjson")
	response, err := elastic.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("bulk index Elasticsearch events: %s: %s", response.Status, strings.TrimSpace(string(body)))
	}
	var result struct {
		Errors bool `json:"errors"`
		Items  []map[string]struct {
			Status int             `json:"status"`
			Error  json.RawMessage `json:"error"`
		} `json:"items"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return err
	}
	if result.Errors {
		for index, item := range result.Items {
			for operation, detail := range item {
				if detail.Status >= 300 {
					return fmt.Errorf("bulk index event %d (%s): %s", index, operation, string(detail.Error))
				}
			}
		}
		return fmt.Errorf("bulk index Elasticsearch events returned item errors")
	}
	return nil
}
