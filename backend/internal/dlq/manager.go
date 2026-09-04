package dlq

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// Stream names
	DefaultDLQStream   = "siem:dead-letter-queue"
	DefaultRetryStream = "siem:retry-queue"

	// DLQ consumer group
	DefaultDLQConsumerGroup   = "dlq-processor"
	DefaultRetryConsumerGroup = "retry-processor"

	// Max retries before moving to permanent DLQ
	MaxRetries = 3

	// Backoff multiplier for exponential retry
	BackoffMultiplier = 2.0
	InitialBackoff    = 5 * time.Second
)

// DeadLetterMessage represents a failed event in DLQ
type DeadLetterMessage struct {
	ID           string            `json:"id"`            // Message ID from stream
	OriginalID   string            `json:"original_id"`   // ID from original stream
	Payload      json.RawMessage   `json:"payload"`       // Original event data
	Error        string            `json:"error"`         // Error message
	FailureCount int               `json:"failure_count"` // Number of retries attempted
	FailedAt     time.Time         `json:"failed_at"`     // When it first failed
	LastAttempt  time.Time         `json:"last_attempt"`  // Last retry attempt
	Source       string            `json:"source"`        // Which stream (ingest, parser, etc.)
	Metadata     map[string]string `json:"metadata"`      // Additional context
}

// Manager handles dead-letter queue operations
type Manager struct {
	redis              *redis.Client
	dlqStream          string
	retryStream        string
	dlqConsumerGroup   string
	retryConsumerGroup string
}

// New creates a new DLQ manager
func New(redisClient *redis.Client) *Manager {
	return NewWithStreams(redisClient, DefaultDLQStream, DefaultRetryStream,
		DefaultDLQConsumerGroup, DefaultRetryConsumerGroup)
}

// NewWithStreams creates a DLQ manager with custom stream names
func NewWithStreams(redisClient *redis.Client, dlqStream, retryStream, dlqGroup, retryGroup string) *Manager {
	return &Manager{
		redis:              redisClient,
		dlqStream:          dlqStream,
		retryStream:        retryStream,
		dlqConsumerGroup:   dlqGroup,
		retryConsumerGroup: retryGroup,
	}
}

// EnsureStreams creates consumer groups if they don't exist
func (m *Manager) EnsureStreams(ctx context.Context) error {
	// Create DLQ consumer group
	_, err := m.redis.XGroupCreateMkStream(ctx, m.dlqStream, m.dlqConsumerGroup, "$").Result()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return fmt.Errorf("create dlq group: %w", err)
	}

	// Create retry consumer group
	_, err = m.redis.XGroupCreateMkStream(ctx, m.retryStream, m.retryConsumerGroup, "$").Result()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return fmt.Errorf("create retry group: %w", err)
	}

	return nil
}

// SendToDLQ sends a failed message to the dead-letter queue
func (m *Manager) SendToDLQ(ctx context.Context, originalID, payload string, err error, source string, metadata map[string]string) (string, error) {
	dlqMsg := DeadLetterMessage{
		OriginalID:   originalID,
		Payload:      json.RawMessage(payload),
		Error:        err.Error(),
		FailureCount: 0,
		FailedAt:     time.Now(),
		LastAttempt:  time.Now(),
		Source:       source,
		Metadata:     metadata,
	}

	msgBytes, err := json.Marshal(dlqMsg)
	if err != nil {
		return "", fmt.Errorf("marshal dlq message: %w", err)
	}

	msgID, err := m.redis.XAdd(ctx, &redis.XAddArgs{
		Stream: m.dlqStream,
		Values: map[string]interface{}{
			"data": string(msgBytes),
		},
	}).Result()
	if err != nil {
		return "", fmt.Errorf("add to dlq stream: %w", err)
	}

	return msgID, nil
}

// SendToRetry sends a message to the retry queue with exponential backoff
func (m *Manager) SendToRetry(ctx context.Context, originalID, payload string, failureCount int, source string, metadata map[string]string) (string, error) {
	// Calculate backoff time
	backoffDuration := time.Duration(math.Pow(BackoffMultiplier, float64(failureCount))) * InitialBackoff
	nextRetryTime := time.Now().Add(backoffDuration)

	dlqMsg := DeadLetterMessage{
		OriginalID:   originalID,
		Payload:      json.RawMessage(payload),
		FailureCount: failureCount,
		FailedAt:     time.Now(),
		LastAttempt:  time.Now(),
		Source:       source,
		Metadata:     metadata,
	}

	msgBytes, err := json.Marshal(dlqMsg)
	if err != nil {
		return "", fmt.Errorf("marshal retry message: %w", err)
	}

	msgID, err := m.redis.XAdd(ctx, &redis.XAddArgs{
		Stream: m.retryStream,
		Values: map[string]interface{}{
			"data":       string(msgBytes),
			"retry_at":   nextRetryTime.Unix(),
			"backoff_ms": backoffDuration.Milliseconds(),
		},
	}).Result()
	if err != nil {
		return "", fmt.Errorf("add to retry stream: %w", err)
	}

	return msgID, nil
}

// GetDLQStats returns statistics about the dead-letter queue
func (m *Manager) GetDLQStats(ctx context.Context) (map[string]interface{}, error) {
	// Get stream length
	length, err := m.redis.XLen(ctx, m.dlqStream).Result()
	if err != nil {
		return nil, fmt.Errorf("get dlq length: %w", err)
	}

	// Get pending messages for consumer group
	pending, err := m.redis.XPending(ctx, m.dlqStream, m.dlqConsumerGroup).Result()
	if err != nil {
		return nil, fmt.Errorf("get dlq pending: %w", err)
	}

	// Count consumers and their pending messages
	consumersCount := len(pending.Consumers)

	return map[string]interface{}{
		"total_messages":    length,
		"pending_messages":  pending.Count,
		"consumers_count":   consumersCount,
		"oldest_message_id": pending.Lower,
		"newest_message_id": pending.Higher,
	}, nil
}

// GetRetryStats returns statistics about the retry queue
func (m *Manager) GetRetryStats(ctx context.Context) (map[string]interface{}, error) {
	// Get stream length
	length, err := m.redis.XLen(ctx, m.retryStream).Result()
	if err != nil {
		return nil, fmt.Errorf("get retry length: %w", err)
	}

	// Get pending messages
	pending, err := m.redis.XPending(ctx, m.retryStream, m.retryConsumerGroup).Result()
	if err != nil {
		return nil, fmt.Errorf("get retry pending: %w", err)
	}

	return map[string]interface{}{
		"total_messages":   length,
		"pending_messages": pending.Count,
		"consumers_count":  len(pending.Consumers),
	}, nil
}

// ConsumeDLQ reads messages from DLQ for processing
// callback is called for each message; if it returns nil, message is acknowledged
func (m *Manager) ConsumeDLQ(ctx context.Context, consumerID string, batchSize int64,
	callback func(context.Context, *DeadLetterMessage) error) error {

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Read pending messages first
		messages, err := m.redis.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    m.dlqConsumerGroup,
			Consumer: consumerID,
			Streams:  []string{m.dlqStream, ">"},
			Count:    batchSize,
			Block:    5 * time.Second,
		}).Result()

		if err != nil && err != redis.Nil {
			return fmt.Errorf("read dlq: %w", err)
		}

		if len(messages) == 0 || len(messages[0].Messages) == 0 {
			continue
		}

		for _, msg := range messages[0].Messages {
			dataStr, ok := msg.Values["data"].(string)
			if !ok {
				if err := m.redis.XAck(ctx, m.dlqStream, m.dlqConsumerGroup, msg.ID).Err(); err != nil {
					return fmt.Errorf("ack invalid dlq message %s: %w", msg.ID, err)
				}
				continue
			}

			var dlqMsg DeadLetterMessage
			if err := json.Unmarshal([]byte(dataStr), &dlqMsg); err != nil {
				// Acknowledge failed parse and continue
				if err := m.redis.XAck(ctx, m.dlqStream, m.dlqConsumerGroup, msg.ID).Err(); err != nil {
					return fmt.Errorf("ack invalid dlq message %s: %w", msg.ID, err)
				}
				continue
			}

			dlqMsg.ID = msg.ID

			// Process message
			if err := callback(ctx, &dlqMsg); err != nil {
				// Keep in pending for retry
				continue
			}

			// Acknowledge on success
			m.redis.XAck(ctx, m.dlqStream, m.dlqConsumerGroup, msg.ID)
		}
	}
}

// ConsumeRetry reads messages from retry queue and reprocesses them
func (m *Manager) ConsumeRetry(ctx context.Context, consumerID string, batchSize int64,
	callback func(context.Context, *DeadLetterMessage) error) error {

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		messages, err := m.redis.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    m.retryConsumerGroup,
			Consumer: consumerID,
			Streams:  []string{m.retryStream, ">"},
			Count:    batchSize,
			Block:    5 * time.Second,
		}).Result()

		if err != nil && err != redis.Nil {
			return fmt.Errorf("read retry: %w", err)
		}

		if len(messages) == 0 || len(messages[0].Messages) == 0 {
			continue
		}

		for _, msg := range messages[0].Messages {
			dataStr, ok := msg.Values["data"].(string)
			if !ok {
				if err := m.redis.XAck(ctx, m.retryStream, m.retryConsumerGroup, msg.ID).Err(); err != nil {
					return fmt.Errorf("ack invalid retry message %s: %w", msg.ID, err)
				}
				continue
			}

			var dlqMsg DeadLetterMessage
			if err := json.Unmarshal([]byte(dataStr), &dlqMsg); err != nil {
				if err := m.redis.XAck(ctx, m.retryStream, m.retryConsumerGroup, msg.ID).Err(); err != nil {
					return fmt.Errorf("ack invalid retry message %s: %w", msg.ID, err)
				}
				continue
			}

			dlqMsg.ID = msg.ID
			if retryAt, ok := msg.Values["retry_at"].(string); ok {
				retryUnix, err := strconv.ParseInt(retryAt, 10, 64)
				if err != nil {
					return fmt.Errorf("parse retry time for %s: %w", msg.ID, err)
				}
				if wait := time.Until(time.Unix(retryUnix, 0)); wait > 0 {
					timer := time.NewTimer(wait)
					select {
					case <-ctx.Done():
						timer.Stop()
						return ctx.Err()
					case <-timer.C:
					}
				}
			}

			// Try processing again
			if err := callback(ctx, &dlqMsg); err != nil {
				// Still failing, check if we should retry or move to permanent DLQ
				if dlqMsg.FailureCount >= MaxRetries {
					// Move to permanent DLQ
					if _, sendErr := m.SendToDLQ(ctx, dlqMsg.OriginalID, string(dlqMsg.Payload),
						fmt.Errorf("max retries exceeded: %w", err), dlqMsg.Source, dlqMsg.Metadata); sendErr != nil {
						return fmt.Errorf("move retry message %s to dlq: %w", msg.ID, sendErr)
					}
					if ackErr := m.redis.XAck(ctx, m.retryStream, m.retryConsumerGroup, msg.ID).Err(); ackErr != nil {
						return fmt.Errorf("ack retry message %s: %w", msg.ID, ackErr)
					}
				} else {
					// Schedule another retry
					if _, sendErr := m.SendToRetry(ctx, dlqMsg.OriginalID, string(dlqMsg.Payload),
						dlqMsg.FailureCount+1, dlqMsg.Source, dlqMsg.Metadata); sendErr != nil {
						return fmt.Errorf("reschedule retry message %s: %w", msg.ID, sendErr)
					}
					if ackErr := m.redis.XAck(ctx, m.retryStream, m.retryConsumerGroup, msg.ID).Err(); ackErr != nil {
						return fmt.Errorf("ack retry message %s: %w", msg.ID, ackErr)
					}
				}
				continue
			}

			// Success
			if err := m.redis.XAck(ctx, m.retryStream, m.retryConsumerGroup, msg.ID).Err(); err != nil {
				return fmt.Errorf("ack retry message %s: %w", msg.ID, err)
			}
		}
	}
}

// ReplayMessage moves a message from DLQ back to retry queue for reprocessing
func (m *Manager) ReplayMessage(ctx context.Context, dlqMessageID string) error {
	// Read the message
	messages, err := m.redis.XRange(ctx, m.dlqStream, dlqMessageID, dlqMessageID).Result()
	if err != nil || len(messages) == 0 {
		return fmt.Errorf("message not found in dlq")
	}

	msg := messages[0]
	dataStr, ok := msg.Values["data"].(string)
	if !ok {
		return fmt.Errorf("invalid message data")
	}

	var dlqMsg DeadLetterMessage
	if err := json.Unmarshal([]byte(dataStr), &dlqMsg); err != nil {
		return fmt.Errorf("unmarshal message: %w", err)
	}

	// Send to retry queue with reset failure count
	_, err = m.SendToRetry(ctx, dlqMsg.OriginalID, string(dlqMsg.Payload),
		0, dlqMsg.Source, dlqMsg.Metadata)
	if err != nil {
		return fmt.Errorf("send to retry: %w", err)
	}

	// Remove from DLQ
	m.redis.XDel(ctx, m.dlqStream, dlqMessageID)

	return nil
}

// PurgeDLQ removes messages older than the specified duration
func (m *Manager) PurgeDLQ(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoffTime := time.Now().Add(-olderThan)
	cutoffTimestamp := cutoffTime.UnixMilli()

	// XTRIM with MINID - convert timestamp to message ID format (timestamp-sequence)
	minID := fmt.Sprintf("%d-0", cutoffTimestamp)
	trimmed, err := m.redis.XTrimMinID(ctx, m.dlqStream, minID).Result()
	if err != nil {
		return 0, fmt.Errorf("trim dlq: %w", err)
	}

	return trimmed, nil
}
