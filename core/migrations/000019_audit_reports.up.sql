CREATE TABLE audit_reports (
    id          UUID PRIMARY KEY     DEFAULT gen_random_uuid(),
    title       TEXT        NOT NULL,
    period_from DATE,
    period_to   DATE,
    summary     TEXT        NOT NULL DEFAULT '',
    findings    JSONB       NOT NULL DEFAULT '[]'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
