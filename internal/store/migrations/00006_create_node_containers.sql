-- +goose Up
CREATE TABLE node_containers (
    node_id TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    database_id UUID NOT NULL REFERENCES databases(id) ON DELETE CASCADE,
    status TEXT NOT NULL,
    port INTEGER NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (node_id, database_id)
);

-- +goose Down
DROP TABLE node_containers;
