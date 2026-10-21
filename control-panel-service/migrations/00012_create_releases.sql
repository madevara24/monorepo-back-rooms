-- +goose Up
CREATE TABLE releases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    version INTEGER NOT NULL UNIQUE,
    commit_sha TEXT NOT NULL,
    checksum TEXT NOT NULL,
    signature TEXT NOT NULL,
    artifact_url TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE releases;
