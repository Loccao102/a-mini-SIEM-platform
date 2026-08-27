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
