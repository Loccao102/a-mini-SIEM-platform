package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// DefaultRateLimit is requests per minute
	DefaultRateLimit = 1000
	DefaultWindow    = 1 * time.Minute
)

// Limiter implements token bucket algorithm using Redis
type Limiter struct {
	redis        *redis.Client
	RequestLimit int64
	window       time.Duration
	keyPrefix    string
}

// New creates a new rate limiter
func New(redisClient *redis.Client, requestLimit int64, window time.Duration) *Limiter {
	if requestLimit == 0 {
		requestLimit = DefaultRateLimit
	}
	if window == 0 {
		window = DefaultWindow
	}
	return &Limiter{
		redis:        redisClient,
		RequestLimit: requestLimit,
		window:       window,
		keyPrefix:    "ratelimit:",
	}
}

// Allow checks if a request from the given identifier is allowed
// Returns (allowed, remaining, resetAt, error)
func (l *Limiter) Allow(ctx context.Context, identifier string) (bool, int64, time.Time, error) {
	key := l.keyPrefix + identifier
	
	// Use Redis to track request count
	val, err := l.redis.Incr(ctx, key).Result()
	if err != nil {
		return false, 0, time.Time{}, fmt.Errorf("rate limit increment: %w", err)
	}
	
	// Set expiration on first request
	if val == 1 {
		l.redis.Expire(ctx, key, l.window)
	}
	
	// Check if limit exceeded
	allowed := val <= l.RequestLimit
	remaining := l.RequestLimit - val
	if remaining < 0 {
		remaining = 0
	}
	
	// Get TTL for reset time
	ttl, err := l.redis.TTL(ctx, key).Result()
	if err != nil {
		ttl = l.window // fallback
	}
	
	resetAt := time.Now().Add(ttl)
	
	return allowed, remaining, resetAt, nil
}

// Reset clears the rate limit for an identifier
func (l *Limiter) Reset(ctx context.Context, identifier string) error {
	key := l.keyPrefix + identifier
	return l.redis.Del(ctx, key).Err()
}

// GetStatus returns current usage statistics for an identifier
func (l *Limiter) GetStatus(ctx context.Context, identifier string) (int64, int64, time.Time, error) {
	key := l.keyPrefix + identifier
	
	val, err := l.redis.Get(ctx, key).Int64()
	if err == redis.Nil {
		return 0, l.RequestLimit, time.Time{}, nil
	}
	if err != nil {
		return 0, 0, time.Time{}, fmt.Errorf("get rate limit status: %w", err)
	}
	
	ttl, err := l.redis.TTL(ctx, key).Result()
	if err != nil {
		return val, l.RequestLimit, time.Time{}, fmt.Errorf("get ttl: %w", err)
	}
	
	resetAt := time.Now().Add(ttl)
	
	return val, l.RequestLimit, resetAt, nil
}
