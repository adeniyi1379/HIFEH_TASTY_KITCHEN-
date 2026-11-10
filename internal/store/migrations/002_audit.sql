CREATE TABLE IF NOT EXISTS audit_log (
    id          TEXT PRIMARY KEY,
    user_id     TEXT,
    user_name   TEXT,
    terminal_id TEXT NOT NULL DEFAULT 'T1',
    action      TEXT NOT NULL,
    entity      TEXT NOT NULL DEFAULT '',
    entity_id   TEXT NOT NULL DEFAULT '',
    details     TEXT NOT NULL DEFAULT '{}',
    at          TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_audit_at ON audit_log (at DESC);
