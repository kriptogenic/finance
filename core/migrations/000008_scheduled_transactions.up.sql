-- Scheduled (recurring) transactions: a transaction template plus a recurrence
-- rule. A background worker materializes due rows into real transactions. The
-- template columns and shape constraint mirror `transactions` (§4/§5) so a
-- materialized row honors the same money invariants.
CREATE TABLE scheduled_transactions (
    id              UUID PRIMARY KEY     DEFAULT gen_random_uuid(),
    name            TEXT,
    type            TEXT        NOT NULL,
    from_account_id UUID        REFERENCES accounts (id) ON DELETE CASCADE,
    to_account_id   UUID        REFERENCES accounts (id) ON DELETE CASCADE,
    category_id     UUID        REFERENCES categories (id) ON DELETE CASCADE,
    amount          BIGINT      NOT NULL,
    to_amount       BIGINT,
    rate_to_base    NUMERIC(20, 10),
    note            TEXT,
    tags            TEXT[]      NOT NULL DEFAULT '{}',

    frequency       TEXT        NOT NULL,
    interval        INTEGER     NOT NULL DEFAULT 1,
    next_run        DATE        NOT NULL,
    end_date        DATE,
    paused          BOOLEAN     NOT NULL DEFAULT FALSE,
    last_run_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT scheduled_transactions_type_chk CHECK (type IN ('expense', 'income', 'transfer')),
    CONSTRAINT scheduled_transactions_amount_chk CHECK (amount > 0),
    CONSTRAINT scheduled_transactions_freq_chk CHECK (frequency IN ('daily', 'weekly', 'monthly', 'yearly')),
    CONSTRAINT scheduled_transactions_interval_chk CHECK (interval > 0),

    -- per-type field applicability (§4) and the transfer/category exclusion (§5)
    CONSTRAINT scheduled_transactions_shape_chk CHECK (
        (type = 'expense'  AND from_account_id IS NOT NULL AND to_account_id IS NULL     AND category_id IS NOT NULL)
        OR (type = 'income'   AND from_account_id IS NULL     AND to_account_id IS NOT NULL AND category_id IS NOT NULL)
        OR (type = 'transfer' AND from_account_id IS NOT NULL AND to_account_id IS NOT NULL AND category_id IS NULL)
    ),
    -- the second leg exists only for transfers
    CONSTRAINT scheduled_transactions_to_leg_chk CHECK (
        type = 'transfer' OR to_amount IS NULL
    )
);

CREATE INDEX scheduled_transactions_next_run_idx ON scheduled_transactions (next_run);
