-- Migration 009: SOAR Playbooks, Executions, and Blocked Entities
-- Enables automated and semi-automated incident response with human-in-the-loop approval and TTL rollback.

CREATE TABLE IF NOT EXISTS playbooks (
    playbook_id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    trigger_type TEXT NOT NULL CHECK (trigger_type IN ('severity', 'category', 'rule_match')),
    trigger_filter JSONB NOT NULL DEFAULT '{}'::jsonb,
    action_type TEXT NOT NULL CHECK (action_type IN ('block_ip', 'isolate_user', 'notify_telegram')),
    action_params JSONB NOT NULL DEFAULT '{}'::jsonb,
    requires_approval BOOLEAN NOT NULL DEFAULT true,
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS playbook_executions (
    execution_id BIGSERIAL PRIMARY KEY,
    playbook_id BIGINT REFERENCES playbooks(playbook_id) ON DELETE SET NULL,
    alert_id BIGINT REFERENCES alerts(alert_id) ON DELETE CASCADE,
    action_type TEXT NOT NULL,
    target TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('pending_approval', 'approved', 'executed', 'rejected', 'failed', 'rolled_back')),
    output JSONB NOT NULL DEFAULT '{}'::jsonb,
    approved_by BIGINT REFERENCES users(user_id),
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    executed_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS blocked_entities (
    blocked_id BIGSERIAL PRIMARY KEY,
    entity_type TEXT NOT NULL CHECK (entity_type IN ('ip', 'user')),
    entity_value TEXT NOT NULL,
    reason TEXT NOT NULL,
    execution_id BIGINT REFERENCES playbook_executions(execution_id) ON DELETE SET NULL,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'released')),
    blocked_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ,
    released_at TIMESTAMPTZ,
    released_by BIGINT REFERENCES users(user_id)
);

CREATE INDEX IF NOT EXISTS idx_playbook_exec_status ON playbook_executions(status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_blocked_entities_val ON blocked_entities(entity_type, entity_value, status);
CREATE INDEX IF NOT EXISTS idx_blocked_entities_exp ON blocked_entities(expires_at, status);

-- Seed default baseline playbooks
INSERT INTO playbooks (name, description, trigger_type, trigger_filter, action_type, action_params, requires_approval, enabled)
VALUES 
    (
        'Auto-Contain SSH Attack Chain IP',
        'Automatically request approval to block source IP involved in multi-stage SSH brute force and privilege escalation',
        'category',
        '{"category": "correlation", "severity": "critical"}'::jsonb,
        'block_ip',
        '{"ttl_seconds": 3600, "firewall_driver": "simulated"}'::jsonb,
        true,
        true
    ),
    (
        'Isolate Compromised Account',
        'Request approval to revoke privileges of account detected in privilege escalation anomaly',
        'rule_match',
        '{"rule_name": "SSH attack chain v1"}'::jsonb,
        'isolate_user',
        '{"action": "disable_account"}'::jsonb,
        true,
        true
    )
ON CONFLICT (name) DO NOTHING;

