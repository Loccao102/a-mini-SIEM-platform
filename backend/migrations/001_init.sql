CREATE TABLE IF NOT EXISTS assets (
    asset_id BIGSERIAL PRIMARY KEY,
    hostname TEXT NOT NULL,
    ip_address INET,
    os_type TEXT NOT NULL,
    criticality TEXT NOT NULL DEFAULT 'medium',
    owner TEXT,
    tags JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS log_sources (
    source_id BIGSERIAL PRIMARY KEY,
    asset_id BIGINT NOT NULL REFERENCES assets(asset_id) ON DELETE CASCADE,
    source_type TEXT NOT NULL,
    agent_id TEXT,
    status TEXT NOT NULL DEFAULT 'active',
    last_seen TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS rules (
    rule_id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    regex_pattern TEXT NOT NULL,
    target_field TEXT NOT NULL,
    condition JSONB NOT NULL DEFAULT '{}'::jsonb,
    severity TEXT NOT NULL DEFAULT 'medium',
    category TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS alerts (
    alert_id BIGSERIAL PRIMARY KEY,
    rule_id BIGINT NOT NULL REFERENCES rules(rule_id),
    asset_id BIGINT REFERENCES assets(asset_id),
    triggered_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen TIMESTAMPTZ NOT NULL DEFAULT now(),
    occurrences INT NOT NULL DEFAULT 1,
    entity_key TEXT,
    severity TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'open',
    assigned_to TEXT,
    summary TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS alert_events (
    alert_id BIGINT NOT NULL REFERENCES alerts(alert_id) ON DELETE CASCADE,
    event_id TEXT NOT NULL,
    PRIMARY KEY (alert_id, event_id)
);

