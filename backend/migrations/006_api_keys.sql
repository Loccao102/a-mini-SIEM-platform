-- Create api_keys table for agent authentication
CREATE TABLE IF NOT EXISTS api_keys (
    api_key_id BIGSERIAL PRIMARY KEY,
    asset_id BIGINT NOT NULL REFERENCES assets(asset_id) ON DELETE CASCADE,
    key_hash VARCHAR(255) NOT NULL UNIQUE,
    display_key VARCHAR(50) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMP,
    last_used_at TIMESTAMP,
    request_count BIGINT NOT NULL DEFAULT 0
);

-- Create indexes for faster lookups
CREATE INDEX IF NOT EXISTS idx_api_keys_key_hash ON api_keys(key_hash);
CREATE INDEX IF NOT EXISTS idx_api_keys_asset_id ON api_keys(asset_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_status ON api_keys(status);
CREATE INDEX IF NOT EXISTS idx_api_keys_asset_status ON api_keys(asset_id, status);
CREATE INDEX IF NOT EXISTS idx_api_keys_expires_at ON api_keys(expires_at) WHERE status = 'active';

-- Add comments
COMMENT ON TABLE api_keys IS 'API keys for agent authentication on ingest endpoint';
COMMENT ON COLUMN api_keys.key_hash IS 'SHA256 hash of the actual API key (not stored plain)';
COMMENT ON COLUMN api_keys.display_key IS 'Safe display format: first 8 chars + ... + last 4 chars';
