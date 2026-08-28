ALTER TABLE alerts ADD COLUMN IF NOT EXISTS entity_key TEXT;
ALTER TABLE alerts ADD COLUMN IF NOT EXISTS occurrences INT NOT NULL DEFAULT 1;
ALTER TABLE alerts ADD COLUMN IF NOT EXISTS last_seen TIMESTAMPTZ NOT NULL DEFAULT now();
CREATE INDEX IF NOT EXISTS idx_alerts_rule_entity ON alerts (rule_id, entity_key, status);
