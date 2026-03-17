CREATE TABLE IF NOT EXISTS events (
    id               TEXT PRIMARY KEY,
    topic            TEXT NOT NULL,
    payload          JSONB NOT NULL,
    idempotency_key  TEXT UNIQUE,
    status           TEXT NOT NULL DEFAULT 'pending',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_events_status ON events(status);
CREATE INDEX IF NOT EXISTS idx_events_topic ON events(topic);
