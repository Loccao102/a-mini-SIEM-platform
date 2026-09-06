-- Existing XOR digests cannot be securely converted without the original key.
-- Run the documented key reissue procedure before rolling out the new backend.
UPDATE api_keys SET status='revoked' WHERE key_hash NOT LIKE 'sha256:%' AND status='active';

CREATE TABLE IF NOT EXISTS rule_event_receipts (
    rule_id BIGINT NOT NULL REFERENCES rules(rule_id) ON DELETE CASCADE,
    event_id TEXT NOT NULL,
    observed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    event_time TIMESTAMPTZ NOT NULL,
    entity_key TEXT NOT NULL,
    PRIMARY KEY (rule_id,event_id)
);
CREATE INDEX IF NOT EXISTS rule_receipts_window ON rule_event_receipts(rule_id,entity_key,observed_at);
CREATE TABLE IF NOT EXISTS detection_events (
    event_id TEXT PRIMARY KEY,
    hostname TEXT NOT NULL,
    username TEXT NOT NULL,
    event_time TIMESTAMPTZ NOT NULL,
    event_type TEXT NOT NULL,
    evidence JSONB NOT NULL
);
CREATE INDEX IF NOT EXISTS detection_window ON detection_events(hostname,username,event_time);
CREATE TABLE IF NOT EXISTS detection_findings (
    finding_id TEXT PRIMARY KEY,
    alert_id BIGINT NOT NULL REFERENCES alerts(alert_id),
    rule_version TEXT NOT NULL,
    techniques JSONB NOT NULL,
    evidence JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO rules(name,description,regex_pattern,target_field,severity,category,enabled)
SELECT 'SSH attack chain v1','3 failures, success, privilege escalation within 10 minutes','a^','message','critical','correlation',false
WHERE NOT EXISTS(SELECT 1 FROM rules WHERE name='SSH attack chain v1');
