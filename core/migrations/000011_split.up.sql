-- Bill splitting. An expense is divided across people: your share stays the
-- expense; each friend's share becomes a transfer into a per-person `receivable`
-- account (a new asset type) that auto-archives once they've paid you back.
-- split_group_id ties the main expense leg to its per-person transfer legs.

ALTER TABLE transactions ADD COLUMN split_group_id UUID;
CREATE INDEX transactions_split_group_idx ON transactions (split_group_id);

-- allow the new `receivable` asset type
ALTER TABLE accounts DROP CONSTRAINT accounts_type_chk;
ALTER TABLE accounts ADD CONSTRAINT accounts_type_chk
    CHECK (type IN ('cash', 'debit_card', 'deposit', 'credit_card', 'loan', 'receivable'));

ALTER TABLE accounts DROP CONSTRAINT accounts_kind_type_chk;
ALTER TABLE accounts ADD CONSTRAINT accounts_kind_type_chk CHECK (
    (kind = 'asset' AND type IN ('cash', 'debit_card', 'deposit', 'receivable'))
    OR (kind = 'liability' AND type IN ('credit_card', 'loan'))
);
