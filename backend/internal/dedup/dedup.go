package dedup

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// TTL cho dedup cache - 5 phút
	DedupTTL = 5 * time.Minute
	// Redis key pattern cho dedup state
	dedupKeyPrefix = "dedup:fp:"
)

type EventGroup struct {
	Fingerprint       string    `json:"fingerprint"`
	Count             int       `json:"duplicate_count"`
	FirstSeen         time.Time `json:"first_seen"`
	LastSeen          time.Time `json:"last_seen"`
	RepresentativeID  string    `json:"representative_event_id"`
}

type Manager struct {
	redis redis.UniversalClient
}

func NewManager(redisClient redis.UniversalClient) *Manager {
	return &Manager{redis: redisClient}
}

// TrackEvent ghi nhận event vào dedup group.
// Trả về group info: count, first_seen, last_seen.
func (m *Manager) TrackEvent(ctx context.Context, fingerprint string, eventID string, eventTime time.Time) (*EventGroup, error) {
	key := dedupKeyPrefix + fingerprint
	
	// Lấy group hiện tại từ Redis
	data, err := m.redis.Get(ctx, key).Result()
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("redis get dedup: %w", err)
	}

	var group EventGroup
	if err == redis.Nil {
		// Nhóm mới
		group = EventGroup{
			Fingerprint:      fingerprint,
			Count:            1,
			FirstSeen:        eventTime,
			LastSeen:         eventTime,
			RepresentativeID: eventID,
		}
	} else {
		// Nhóm đã tồn tại, increment count
		if err := json.Unmarshal([]byte(data), &group); err != nil {
			return nil, fmt.Errorf("dedup unmarshal: %w", err)
		}
		group.Count++
		group.LastSeen = eventTime
		// Giữ nguyên representative (event đầu tiên trong group)
	}

	// Lưu lại vào Redis với TTL
	serialized, err := json.Marshal(group)
	if err != nil {
		return nil, fmt.Errorf("dedup marshal: %w", err)
	}
	if err := m.redis.Set(ctx, key, serialized, DedupTTL).Err(); err != nil {
		return nil, fmt.Errorf("redis set dedup: %w", err)
	}

	return &group, nil
}

// GetGroup lấy thông tin nhóm hiện tại (không update).
func (m *Manager) GetGroup(ctx context.Context, fingerprint string) (*EventGroup, error) {
	key := dedupKeyPrefix + fingerprint
	data, err := m.redis.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Nhóm không tồn tại
		}
		return nil, fmt.Errorf("redis get dedup: %w", err)
	}

	var group EventGroup
	if err := json.Unmarshal([]byte(data), &group); err != nil {
		return nil, fmt.Errorf("dedup unmarshal: %w", err)
	}
	return &group, nil
}

// DeleteGroup xóa dedup state (cleanup).
func (m *Manager) DeleteGroup(ctx context.Context, fingerprint string) error {
	key := dedupKeyPrefix + fingerprint
	return m.redis.Del(ctx, key).Err()
}
