package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, elastic.baseURL+"/normalized_events/_doc", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := elastic.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return fmt.Errorf("index Elasticsearch event: %s", response.Status)
	}
	return nil
}
