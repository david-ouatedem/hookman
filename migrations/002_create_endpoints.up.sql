CREATE TABLE IF NOT EXISTS endpoints (
    id              TEXT PRIMARY KEY,
    url             TEXT NOT NULL,
    topics          TEXT[] NOT NULL,
    description     TEXT,
    signing_secret  TEXT NOT NULL,
    enabled         BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
