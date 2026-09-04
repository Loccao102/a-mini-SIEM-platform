ALTER TABLE assets
    ADD COLUMN IF NOT EXISTS agent_status TEXT NOT NULL DEFAULT 'unknown' CHECK (agent_status IN ('unknown', 'healthy', 'unhealthy', 'inactive')),
    ADD COLUMN IF NOT EXISTS last_seen TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS enrolled_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS policy_name TEXT,
    ADD COLUMN IF NOT EXISTS policy_version INTEGER;

CREATE INDEX IF NOT EXISTS idx_assets_agent_status_seen ON assets(agent_status, last_seen);
CREATE INDEX IF NOT EXISTS idx_log_sources_agent_status_seen ON log_sources(agent_id, status, last_seen);

CREATE TABLE IF NOT EXISTS fleet_policies (
    policy_id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    os_type TEXT NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    config JSONB NOT NULL DEFAULT '{}'::jsonb,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'retired')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS fleet_policy_deployments (
    deployment_id BIGSERIAL PRIMARY KEY,
    policy_id BIGINT NOT NULL REFERENCES fleet_policies(policy_id),
    asset_id BIGINT NOT NULL REFERENCES assets(asset_id) ON DELETE CASCADE,
    agent_id TEXT NOT NULL,
    policy_version INTEGER NOT NULL,
    status TEXT NOT NULL DEFAULT 'deployed' CHECK (status IN ('pending', 'deployed', 'failed', 'rolled_back')),
    deployed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    details JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS idx_fleet_deployments_asset ON fleet_policy_deployments(asset_id, deployed_at DESC);
CREATE INDEX IF NOT EXISTS idx_fleet_deployments_policy ON fleet_policy_deployments(policy_id, policy_version, deployed_at DESC);

CREATE TABLE IF NOT EXISTS pipeline_alerts (
    pipeline_alert_id BIGSERIAL PRIMARY KEY,
    alert_key TEXT NOT NULL UNIQUE,
    severity TEXT NOT NULL CHECK (severity IN ('warning', 'critical')),
    status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'resolved')),
    queue_name TEXT NOT NULL,
    observed_value BIGINT NOT NULL,
    threshold BIGINT NOT NULL,
    message TEXT NOT NULL,
    first_seen TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_pipeline_alerts_status ON pipeline_alerts(status, last_seen DESC);
