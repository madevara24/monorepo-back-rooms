-- +goose Up
CREATE TABLE plans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    size_gb INTEGER NOT NULL CHECK (size_gb BETWEEN 1 AND 20),
    price_idr_month BIGINT NOT NULL CHECK (price_idr_month >= 0),
    tagline TEXT NOT NULL DEFAULT '',
    badge TEXT NOT NULL DEFAULT '',
    sort_order INTEGER NOT NULL DEFAULT 0,
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE databases ADD COLUMN plan_id UUID REFERENCES plans(id) ON DELETE SET NULL;
ALTER TABLE databases ADD COLUMN price_idr_month BIGINT;
CREATE INDEX databases_plan_id_idx ON databases(plan_id);

INSERT INTO plans (slug, name, size_gb, price_idr_month, tagline, badge, sort_order) VALUES
    ('sandbox', 'Sandbox', 1, 10000, 'For the project you swear you''ll finish.', '', 1),
    ('side-project', 'Side project', 5, 50000, 'Real data, no real users yet.', '', 2),
    ('production', 'Production', 10, 100000, 'The one you actually put behind a DNS record.', 'Most picked', 3),
    ('heavy', 'Heavy', 20, 200000, 'For when the indexes got out of hand.', '', 4);

-- +goose Down
DROP INDEX databases_plan_id_idx;
ALTER TABLE databases DROP COLUMN plan_id;
ALTER TABLE databases DROP COLUMN price_idr_month;
DROP TABLE plans;
