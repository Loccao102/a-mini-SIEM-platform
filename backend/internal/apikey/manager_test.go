package apikey

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func setupTestDB(t *testing.T) *pgxpool.Pool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, "postgres://siem:siem_dev_password@localhost:5432/siem?sslmode=disable")
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}

	// Create test asset for API key tests
	_, err = pool.Exec(ctx, `INSERT INTO assets (hostname, ip_address, os_type, criticality) 
		VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING`,
		"test-agent-apikey", "192.168.1.100", "linux", "high")
	if err != nil {
		t.Fatalf("create test asset: %v", err)
	}

	return pool
}

func cleanupTestDB(t *testing.T, pool *pgxpool.Pool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool.Exec(ctx, `DELETE FROM api_keys WHERE key_hash LIKE 'test%'`)
	pool.Close()
}

func TestGenerateKey(t *testing.T) {
	pool := setupTestDB(t)
	defer cleanupTestDB(t, pool)
	ctx := context.Background()

	mgr := New(pool)

	// Get test asset ID
	var assetID int64
	err := pool.QueryRow(ctx, `SELECT asset_id FROM assets WHERE hostname = $1`, "test-agent-apikey").Scan(&assetID)
	if err != nil {
		t.Fatalf("get test asset: %v", err)
	}

	// Generate key without expiration
	apiKey, rawKey, err := mgr.GenerateKey(ctx, assetID, nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	if apiKey.KeyID == 0 {
		t.Error("KeyID should be set")
	}
	if apiKey.Status != "active" {
		t.Errorf("Status should be 'active', got %q", apiKey.Status)
	}
	if apiKey.AssetID != assetID {
		t.Errorf("AssetID mismatch: expected %d, got %d", assetID, apiKey.AssetID)
	}
	if !startsWith(rawKey, KeyPrefix) {
		t.Errorf("Raw key should start with %q, got %q", KeyPrefix, rawKey[:len(KeyPrefix)])
	}
	if apiKey.ExpiresAt != nil {
		t.Error("ExpiresAt should be nil when no expiration specified")
	}
}

func TestGenerateKeyWithExpiration(t *testing.T) {
	pool := setupTestDB(t)
	defer cleanupTestDB(t, pool)
	ctx := context.Background()

	mgr := New(pool)

	var assetID int64
	pool.QueryRow(ctx, `SELECT asset_id FROM assets WHERE hostname = $1`, "test-agent-apikey").Scan(&assetID)

	expiration := 24 * time.Hour
	apiKey, _, err := mgr.GenerateKey(ctx, assetID, &expiration)
	if err != nil {
		t.Fatalf("generate key with expiration: %v", err)
	}

	if apiKey.ExpiresAt == nil {
		t.Error("ExpiresAt should be set")
	} else {
		// Allow 1 minute tolerance
		now := time.Now()
		if apiKey.ExpiresAt.Before(now.Add(23*time.Hour)) || apiKey.ExpiresAt.After(now.Add(25*time.Hour)) {
			t.Errorf("ExpiresAt should be approximately 24 hours from now")
		}
	}
}

func TestValidateKey(t *testing.T) {
	pool := setupTestDB(t)
	defer cleanupTestDB(t, pool)
	ctx := context.Background()

	mgr := New(pool)

	var assetID int64
	pool.QueryRow(ctx, `SELECT asset_id FROM assets WHERE hostname = $1`, "test-agent-apikey").Scan(&assetID)

	// Generate a key
	_, rawKey, err := mgr.GenerateKey(ctx, assetID, nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	// Validate the key
	apiKey, err := mgr.ValidateKey(ctx, rawKey)
	if err != nil {
		t.Fatalf("validate key: %v", err)
	}

	if apiKey.AssetID != assetID {
		t.Errorf("AssetID mismatch: expected %d, got %d", assetID, apiKey.AssetID)
	}
	if apiKey.Status != "active" {
		t.Errorf("Status should be 'active', got %q", apiKey.Status)
	}
}

func TestValidateKeyInvalid(t *testing.T) {
	pool := setupTestDB(t)
	defer cleanupTestDB(t, pool)
	ctx := context.Background()

	mgr := New(pool)

	// Try to validate an invalid key
	_, err := mgr.ValidateKey(ctx, "sk_invalid_key")
	if err == nil {
		t.Error("ValidateKey should return error for invalid key")
	}
}

func TestRevokeKey(t *testing.T) {
	pool := setupTestDB(t)
	defer cleanupTestDB(t, pool)
	ctx := context.Background()

	mgr := New(pool)

	var assetID int64
	pool.QueryRow(ctx, `SELECT asset_id FROM assets WHERE hostname = $1`, "test-agent-apikey").Scan(&assetID)

	// Generate and revoke a key
	apiKey, _, err := mgr.GenerateKey(ctx, assetID, nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	err = mgr.RevokeKey(ctx, apiKey.KeyID)
	if err != nil {
		t.Fatalf("revoke key: %v", err)
	}

	// Verify status changed
	var status string
	pool.QueryRow(ctx, `SELECT status FROM api_keys WHERE api_key_id = $1`, apiKey.KeyID).Scan(&status)
	if status != "revoked" {
		t.Errorf("Status should be 'revoked', got %q", status)
	}
}

func TestListKeys(t *testing.T) {
	pool := setupTestDB(t)
	defer cleanupTestDB(t, pool)
	ctx := context.Background()

	mgr := New(pool)

	var assetID int64
	pool.QueryRow(ctx, `SELECT asset_id FROM assets WHERE hostname = $1`, "test-agent-apikey").Scan(&assetID)

	// Generate multiple keys
	_, _, err := mgr.GenerateKey(ctx, assetID, nil)
	if err != nil {
		t.Fatalf("generate key 1: %v", err)
	}

	_, _, err = mgr.GenerateKey(ctx, assetID, nil)
	if err != nil {
		t.Fatalf("generate key 2: %v", err)
	}

	// List keys
	keys, err := mgr.ListKeys(ctx, assetID)
	if err != nil {
		t.Fatalf("list keys: %v", err)
	}

	if len(keys) < 2 {
		t.Errorf("Expected at least 2 keys, got %d", len(keys))
	}
}

func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
