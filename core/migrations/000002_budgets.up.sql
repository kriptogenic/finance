-- Per-category spending budgets (REQUIREMENTS Appendix A / Budget). amount is in
-- base currency minor units. One budget per category; covers its subcategories.
CREATE TABLE budgets (
    id           UUID PRIMARY KEY     DEFAULT gen_random_uuid(),
    category_id  UUID        NOT NULL UNIQUE REFERENCES categories (id) ON DELETE CASCADE,
    period       TEXT        NOT NULL DEFAULT 'monthly',
    amount       BIGINT      NOT NULL,
    rollover     BOOLEAN     NOT NULL DEFAULT FALSE,
    start_period DATE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT budgets_period_chk CHECK (period IN ('weekly', 'monthly', 'yearly')),
    CONSTRAINT budgets_amount_chk CHECK (amount > 0)
);
