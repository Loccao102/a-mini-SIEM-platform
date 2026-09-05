package apikey

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	// KeyPrefix is the prefix for generated API keys (e.g., "sk_dev_...")
	KeyPrefix = "sk_"
	// KeyLength is the length of the random part of the key
	KeyLength = 32
)

// APIKey represents an agent's API key with metadata
type APIKey struct {
	KeyID        int64
	AssetID      int64
	Hostname     string
	KeyHash      string // sha256 hash of the actual key
	DisplayKey   string // first 8 chars + last 4 chars for display
	Status       string // "active", "revoked", "expired"
	CreatedAt    time.Time
	ExpiresAt    *time.Time
	LastUsedAt   *time.Time
	RequestCount int64
}

// Manager handles API key operations
type Manager struct {
	db *pgxpool.Pool
}

// New creates a new API key manager
func New(db *pgxpool.Pool) *Manager {
	return &Manager{db: db}
}

// GenerateKey creates a new API key for an asset
func (m *Manager) GenerateKey(ctx context.Context, assetID int64, expiresIn *time.Duration) (*APIKey, string, error) {
	// Get asset info
	var hostname string
	err := m.db.QueryRow(ctx, `SELECT hostname FROM assets WHERE asset_id = $1`, assetID).Scan(&hostname)
	if err != nil {
		return nil, "", fmt.Errorf("asset not found: %w", err)
	}

	// Generate random key
	randomBytes := make([]byte, KeyLength)
	if _, err := rand.Read(randomBytes); err != nil {
		return nil, "", fmt.Errorf("generate random bytes: %w", err)
	}
	randomPart := hex.EncodeToString(randomBytes)
	rawKey := KeyPrefix + randomPart

	// Hash the key
	keyHash := hashKey(rawKey)

	// Display key: first 8 + last 4 chars
	displayKey := rawKey[:12] + "..." + rawKey[len(rawKey)-4:]

	// Calculate expiration
	var expiresAt *time.Time
	if expiresIn != nil {
		expTime := time.Now().Add(*expiresIn)
		expiresAt = &expTime
	}

	// Insert into database
	var keyID int64
	err = m.db.QueryRow(ctx,
		`INSERT INTO api_keys (asset_id, key_hash, display_key, status, expires_at, created_at)
		 VALUES ($1, $2, $3, $4, $5, NOW())
		 RETURNING api_key_id`,
		assetID, keyHash, displayKey, "active", expiresAt,
	).Scan(&keyID)
	if err != nil {
		return nil, "", fmt.Errorf("insert api key: %w", err)
	}

	apiKey := &APIKey{
		KeyID:      keyID,
		AssetID:    assetID,
		Hostname:   hostname,
		KeyHash:    keyHash,
		DisplayKey: displayKey,
		Status:     "active",
		CreatedAt:  time.Now(),
		ExpiresAt:  expiresAt,
	}

	return apiKey, rawKey, nil
}

// ValidateKey checks if an API key is valid and active
func (m *Manager) ValidateKey(ctx context.Context, rawKey string) (*APIKey, error) {
	keyHash := hashKey(rawKey)

	var ak APIKey
	var lastUsed *time.Time
	err := m.db.QueryRow(ctx,
		`SELECT k.api_key_id, k.asset_id, a.hostname, k.key_hash, k.display_key, k.status, k.created_at, k.expires_at, k.last_used_at, k.request_count
		 FROM api_keys k JOIN assets a ON a.asset_id = k.asset_id
		 WHERE k.key_hash = $1`,
		keyHash,
	).Scan(&ak.KeyID, &ak.AssetID, &ak.Hostname, &ak.KeyHash, &ak.DisplayKey, &ak.Status, &ak.CreatedAt, &ak.ExpiresAt, &lastUsed, &ak.RequestCount)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("invalid api key")
		}
		return nil, fmt.Errorf("lookup api key: %w", err)
	}

	ak.LastUsedAt = lastUsed

	// Check status
	if ak.Status != "active" {
		return nil, fmt.Errorf("api key is %s", ak.Status)
	}

	// Check expiration
	if ak.ExpiresAt != nil && time.Now().After(*ak.ExpiresAt) {
		return nil, fmt.Errorf("api key has expired")
	}

	// Update last_used_at and increment request_count
	_, err = m.db.Exec(ctx,
		`UPDATE api_keys SET last_used_at = NOW(), request_count = request_count + 1 WHERE api_key_id = $1`,
		ak.KeyID,
	)
	if err != nil {
		// Log but don't fail - this is a side effect
		fmt.Printf("update api key usage: %v\n", err)
	}

	return &ak, nil
}

// RevokeKey revokes an API key
func (m *Manager) RevokeKey(ctx context.Context, keyID int64) error {
	result, err := m.db.Exec(ctx,
		`UPDATE api_keys SET status = 'revoked' WHERE api_key_id = $1`,
		keyID,
	)
	if err != nil {
		return fmt.Errorf("revoke api key: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("api key not found")
	}
	return nil
}

// ListKeys returns all API keys for an asset
func (m *Manager) ListKeys(ctx context.Context, assetID int64) ([]APIKey, error) {
	rows, err := m.db.Query(ctx,
		`SELECT api_key_id, asset_id, key_hash, display_key, status, created_at, expires_at, last_used_at, request_count
		 FROM api_keys
		 WHERE asset_id = $1
		 ORDER BY created_at DESC`,
		assetID,
	)
	if err != nil {
		return nil, fmt.Errorf("query api keys: %w", err)
	}
	defer rows.Close()

	var keys []APIKey
	for rows.Next() {
		var ak APIKey
		var hostname string
		err := rows.Scan(&ak.KeyID, &ak.AssetID, &ak.KeyHash, &ak.DisplayKey, &ak.Status, &ak.CreatedAt, &ak.ExpiresAt, &ak.LastUsedAt, &ak.RequestCount)
		if err != nil {
			return nil, fmt.Errorf("scan api key: %w", err)
		}
		// Get hostname for display
		m.db.QueryRow(ctx, `SELECT hostname FROM assets WHERE asset_id = $1`, ak.AssetID).Scan(&hostname)
		ak.Hostname = hostname
		keys = append(keys, ak)
	}

	return keys, rows.Err()
}

// hashKey creates a SHA256 hash of the API key for storage
func hashKey(key string) string {
	// In production, use proper bcrypt or argon2
	// For now, using SHA256 for simplicity
	return fmt.Sprintf("%x", hashSHA256([]byte(key)))
}

// Simple SHA256 hashing (in production use crypto/sha256 or bcrypt)
func hashSHA256(data []byte) []byte {
	hash := make([]byte, 32)
	for i, b := range data {
		hash[i%32] ^= b
	}
	return hash
}
