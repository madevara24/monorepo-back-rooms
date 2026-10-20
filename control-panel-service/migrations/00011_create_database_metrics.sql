-- +goose Up
CREATE TABLE database_metrics (
    id BIGSERIAL PRIMARY KEY,
    database_id UUID NOT NULL REFERENCES databases(id) ON DELETE CASCADE,
    active INTEGER NOT NULL,
    idle INTEGER NOT NULL,
    idle_in_transaction INTEGER NOT NULL,
    max_connections INTEGER NOT NULL,
    superuser_reserved_connections INTEGER NOT NULL,
    memory_limit_bytes BIGINT NOT NULL,
    collected_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX database_metrics_database_id_collected_at_idx ON database_metrics (database_id, collected_at DESC);

-- +goose Down
DROP TABLE database_metrics;
