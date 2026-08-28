package ingest

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

const DefaultStream = "siem:raw-logs"

type Client struct {
	redis  redis.UniversalClient
	stream string
}

type Message struct {
	Raw        string
	SourceType string
	Hostname   string
	AgentID    string
	ReceivedAt time.Time
}

type StreamStatus struct {
	StreamLength int64  `json:"stream_length"`
	Pending      int64  `json:"pending"`
	Consumers    int64  `json:"consumers"`
	LastID       string `json:"last_id"`
}

func NewClient(redisURL, stream string) (*Client, error) {
	options, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	return &Client{redis: redis.NewClient(options), stream: stream}, nil
}

func (client *Client) Ping(ctx context.Context) error {
	return client.redis.Ping(ctx).Err()
}

func (client *Client) Status(ctx context.Context, group string) (StreamStatus, error) {
	stream, err := client.redis.XInfoStream(ctx, client.stream).Result()
	if err != nil {
		if err == redis.Nil {
			return StreamStatus{}, nil
		}
		return StreamStatus{}, err
	}
	status := StreamStatus{StreamLength: stream.Length, LastID: stream.LastGeneratedID}
	groups, err := client.redis.XInfoGroups(ctx, client.stream).Result()
	if err != nil {
		return StreamStatus{}, err
	}
	for _, current := range groups {
		if current.Name == group {
			status.Pending = current.Pending
			status.Consumers = current.Consumers
			break
		}
	}
	return status, nil
}

func (client *Client) Publish(ctx context.Context, message Message) (string, error) {
	return client.redis.XAdd(ctx, &redis.XAddArgs{
		Stream: client.stream,
		Values: map[string]any{
			"raw":         message.Raw,
			"source_type": message.SourceType,
			"hostname":    message.Hostname,
			"agent_id":    message.AgentID,
			"received_at": message.ReceivedAt.UTC().Format(time.RFC3339Nano),
		},
	}).Result()
}

func (client *Client) Close() error {
	if redisClient, ok := client.redis.(*redis.Client); ok {
		return redisClient.Close()
	}
	return nil
}
