-- +goose Up
CREATE TABLE databases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    size_gb INTEGER NOT NULL,
    status TEXT NOT NULL DEFAULT 'provisioning',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX databases_user_id_idx ON databases(user_id);

-- +goose Down
DROP TABLE databases;
