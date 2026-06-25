ALTER TABLE accounts DROP CONSTRAINT accounts_kind_type_chk;
ALTER TABLE accounts ADD CONSTRAINT accounts_kind_type_chk CHECK (
    (kind = 'asset' AND type IN ('cash', 'debit_card', 'deposit'))
    OR (kind = 'liability' AND type IN ('credit_card', 'loan'))
);

ALTER TABLE accounts DROP CONSTRAINT accounts_type_chk;
ALTER TABLE accounts ADD CONSTRAINT accounts_type_chk
    CHECK (type IN ('cash', 'debit_card', 'deposit', 'credit_card', 'loan'));

DROP INDEX transactions_split_group_idx;
ALTER TABLE transactions DROP COLUMN split_group_id;
