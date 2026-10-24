-- +goose Up
CREATE TABLE demo_state (
    id BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (id),
    generation BIGINT NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'idle',
    maintenance BOOLEAN NOT NULL DEFAULT FALSE,
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    next_reset_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_error TEXT
);

INSERT INTO demo_state (id) VALUES (TRUE);

CREATE TABLE reset_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    trigger_kind TEXT NOT NULL,
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ,
    containers_deleted INTEGER NOT NULL DEFAULT 0,
    volumes_deleted INTEGER NOT NULL DEFAULT 0,
    orphans_deleted INTEGER NOT NULL DEFAULT 0,
    error TEXT
);

ALTER TABLE users ADD COLUMN is_seed BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE plans ADD COLUMN purchasable BOOLEAN NOT NULL DEFAULT TRUE;

UPDATE plans SET purchasable = FALSE WHERE slug IN ('production', 'heavy');

-- +goose Down
ALTER TABLE plans DROP COLUMN purchasable;
ALTER TABLE users DROP COLUMN is_seed;
DROP TABLE reset_runs;
DROP TABLE demo_state;
