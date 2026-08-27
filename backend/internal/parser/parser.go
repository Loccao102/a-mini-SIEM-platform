package parser

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/redis/go-redis/v9"
)

const DefaultConsumerGroup = "siem-parser"

var failedSSHLogin = regexp.MustCompile(`Failed password for (?:invalid user )?(\S+) from (\S+) port (\d+)`)
var windowsLogonFailure = regexp.MustCompile(`(?i)(?:An account failed to log on|logon failure).*?Account Name:\s*(\S+).*?Source Network Address:\s*(\S+)`)
var nginxAccess = regexp.MustCompile(`^(\S+) \S+ \S+ \[[^]]+\] "(\S+) ([^"]+)" (\d{3}) (\d+)`)

type NormalizedEvent struct {
	EventID     string            `json:"event_id"`
	EventTime   time.Time         `json:"event_time"`
	EventType   string            `json:"event_type"`
	LogCategory string            `json:"log_category"`
	Severity    string            `json:"severity"`
	SrcIP       string            `json:"src_ip,omitempty"`
	Username    string            `json:"username,omitempty"`
	Message     string            `json:"message"`
	Hostname    string            `json:"hostname,omitempty"`
	AgentID     string            `json:"agent_id,omitempty"`
	Extra       map[string]string `json:"extra_fields,omitempty"`
}

func Parse(message redis.XMessage) NormalizedEvent {
	raw := value(message.Values, "raw")
	event := NormalizedEvent{EventID: message.ID, EventTime: parseTime(value(message.Values, "received_at")), EventType: "log", LogCategory: value(message.Values, "source_type"), Severity: "info", Message: raw, Hostname: value(message.Values, "hostname"), AgentID: value(message.Values, "agent_id")}
	if matches := failedSSHLogin.FindStringSubmatch(raw); matches != nil {
		event.EventType, event.LogCategory, event.Severity = "authentication_failure", "linux_sshd", "medium"
		event.Username, event.SrcIP = matches[1], matches[2]
		event.Extra = map[string]string{"src_port": matches[3]}
	} else if matches := windowsLogonFailure.FindStringSubmatch(raw); matches != nil {
		event.EventType, event.LogCategory, event.Severity = "authentication_failure", "windows_security", "medium"
		event.Username, event.SrcIP = matches[1], matches[2]
	} else if matches := nginxAccess.FindStringSubmatch(raw); matches != nil {
		event.EventType, event.LogCategory = "http_access", "nginx"
		event.SrcIP = matches[1]
		event.Extra = map[string]string{"method": matches[2], "path": matches[3], "status_code": matches[4], "bytes": matches[5]}
	}
	return event
}

type Consumer struct {
	redis               redis.UniversalClient
	stream, group, name string
}

func NewConsumer(redisURL, stream, group, name string) (*Consumer, error) {
	options, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	return &Consumer{redis: redis.NewClient(options), stream: stream, group: group, name: name}, nil
}

func (consumer *Consumer) Close() error {
	if client, ok := consumer.redis.(*redis.Client); ok {
		return client.Close()
	}
	return nil
}

func (consumer *Consumer) EnsureGroup(ctx context.Context) error {
	err := consumer.redis.XGroupCreateMkStream(ctx, consumer.stream, consumer.group, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return err
	}
	return nil
}

func (consumer *Consumer) Consume(ctx context.Context, handler func(context.Context, NormalizedEvent) error) error {
	if err := consumer.EnsureGroup(ctx); err != nil {
		return err
	}
	for {
		streams, err := consumer.redis.XReadGroup(ctx, &redis.XReadGroupArgs{Group: consumer.group, Consumer: consumer.name, Streams: []string{consumer.stream, ">"}, Count: 10, Block: time.Second}).Result()
		if err != nil {
			if err == redis.Nil {
				continue
			}
			return err
		}
		for _, stream := range streams {
			for _, message := range stream.Messages {
				event := Parse(message)
				if err := handler(ctx, event); err != nil {
					return fmt.Errorf("handle event %s: %w", message.ID, err)
				}
				if err := consumer.redis.XAck(ctx, consumer.stream, consumer.group, message.ID).Err(); err != nil {
					return fmt.Errorf("ack event %s: %w", message.ID, err)
				}
			}
		}
	}
}

func value(values map[string]any, key string) string {
	if value, ok := values[key]; ok {
		return fmt.Sprint(value)
	}
	return ""
}

func parseTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Now().UTC()
	}
	return parsed
}
