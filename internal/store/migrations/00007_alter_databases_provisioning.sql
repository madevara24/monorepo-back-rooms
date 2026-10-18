-- +goose Up
ALTER TABLE databases
    ADD COLUMN slug TEXT,
    ADD COLUMN node_id TEXT REFERENCES nodes(id),
    ADD COLUMN host TEXT,
    ADD COLUMN port INTEGER,
    ADD COLUMN password_encrypted BYTEA,
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

CREATE UNIQUE INDEX databases_slug_idx ON databases(slug);

-- +goose Down
DROP INDEX databases_slug_idx;
ALTER TABLE databases
    DROP COLUMN slug,
    DROP COLUMN node_id,
    DROP COLUMN host,
    DROP COLUMN port,
    DROP COLUMN password_encrypted,
    DROP COLUMN updated_at;
