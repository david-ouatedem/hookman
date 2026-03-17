CREATE TABLE IF NOT EXISTS delivery_attempts (
    id              TEXT PRIMARY KEY,
    event_id        TEXT NOT NULL REFERENCES events(id),
    endpoint_id     TEXT NOT NULL REFERENCES endpoints(id),
    attempt_number  INTEGER NOT NULL,
    http_status     INTEGER,
    response_body   TEXT,
    duration_ms     INTEGER,
    status          TEXT NOT NULL,
    attempted_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    next_retry_at   TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_delivery_attempts_event_id ON delivery_attempts(event_id);
CREATE INDEX IF NOT EXISTS idx_delivery_attempts_next_retry ON delivery_attempts(next_retry_at)
    WHERE next_retry_at IS NOT NULL;
