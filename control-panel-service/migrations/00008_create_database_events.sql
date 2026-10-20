-- +goose Up
CREATE TABLE database_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    database_id UUID NOT NULL REFERENCES databases(id) ON DELETE CASCADE,
    from_status TEXT,
    to_status TEXT NOT NULL,
    reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX database_events_database_id_idx ON database_events(database_id);

-- +goose Down
DROP TABLE database_events;
