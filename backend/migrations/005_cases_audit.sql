CREATE TABLE IF NOT EXISTS cases (
    case_id BIGSERIAL PRIMARY KEY,
    title TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'investigating', 'resolved', 'closed')),
    classification TEXT CHECK (classification IN ('true_positive', 'false_positive')),
    priority TEXT NOT NULL DEFAULT 'medium' CHECK (priority IN ('low', 'medium', 'high', 'critical')),
    assigned_to BIGINT REFERENCES users(user_id),
    created_by BIGINT NOT NULL REFERENCES users(user_id),
    resolution TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS case_alerts (
    case_id BIGINT NOT NULL REFERENCES cases(case_id) ON DELETE CASCADE,
    alert_id BIGINT NOT NULL REFERENCES alerts(alert_id) ON DELETE CASCADE,
    added_by BIGINT NOT NULL REFERENCES users(user_id),
    added_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (case_id, alert_id)
);

CREATE TABLE IF NOT EXISTS case_notes (
    note_id BIGSERIAL PRIMARY KEY,
    case_id BIGINT NOT NULL REFERENCES cases(case_id) ON DELETE CASCADE,
    author_user_id BIGINT NOT NULL REFERENCES users(user_id),
    body TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS audit_logs (
    audit_id BIGSERIAL PRIMARY KEY,
    actor_user_id BIGINT REFERENCES users(user_id),
    action TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id BIGINT,
    details JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_cases_status ON cases(status, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_case_alerts_alert ON case_alerts(alert_id);
CREATE INDEX IF NOT EXISTS idx_case_notes_case ON case_notes(case_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_entity ON audit_logs(entity_type, entity_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created ON audit_logs(created_at DESC);
