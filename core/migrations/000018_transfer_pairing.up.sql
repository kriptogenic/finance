CREATE TABLE consumed_legs (
    external_id    TEXT PRIMARY KEY,
    transaction_id UUID        NOT NULL REFERENCES transactions (id) ON DELETE CASCADE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX transactions_pair_idx
    ON transactions (type, ((amount).amount), date)
    WHERE external_id IS NOT NULL AND split_group_id IS NULL;
