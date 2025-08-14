CREATE TABLE IF NOT EXISTS events (
    event_id UUID PRIMARY KEY,
    aggregate_id VARCHAR(255) NOT NULL,
    aggregate_type VARCHAR(255) NOT NULL,
    event_type VARCHAR(255) NOT NULL,
    aggregate_version INT NOT NULL,
    payload JSONB NOT NULL,
    "timestamp" TIMESTAMPTZ NOT NULL,
    correlation_id VARCHAR(255),
    user_id VARCHAR(255),
    -- Ensures that the version for a given aggregate is always unique
    CONSTRAINT unique_aggregate_version UNIQUE (aggregate_id, aggregate_version)
);

CREATE INDEX IF NOT EXISTS idx_events_aggregate_id ON events (aggregate_id);