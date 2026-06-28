-- Reverse money_t back into separate (amount, currency) columns.

-- receipts + items
ALTER TABLE receipt_items
    ALTER COLUMN price DROP DEFAULT,
    ALTER COLUMN vat_amount DROP DEFAULT,
    ALTER COLUMN discount DROP DEFAULT,
    ALTER COLUMN other DROP DEFAULT,
    ALTER COLUMN price TYPE bigint USING (price).amount,
    ALTER COLUMN vat_amount TYPE bigint USING (vat_amount).amount,
    ALTER COLUMN discount TYPE bigint USING (discount).amount,
    ALTER COLUMN other TYPE bigint USING (other).amount,
    ALTER COLUMN price SET DEFAULT 0,
    ALTER COLUMN vat_amount SET DEFAULT 0,
    ALTER COLUMN discount SET DEFAULT 0,
    ALTER COLUMN other SET DEFAULT 0;

ALTER TABLE receipts
    ALTER COLUMN paid_cash DROP DEFAULT,
    ALTER COLUMN paid_card DROP DEFAULT,
    ALTER COLUMN total_amount DROP DEFAULT,
    ALTER COLUMN total_vat DROP DEFAULT,
    ALTER COLUMN paid_cash TYPE bigint USING (paid_cash).amount,
    ALTER COLUMN paid_card TYPE bigint USING (paid_card).amount,
    ALTER COLUMN total_amount TYPE bigint USING (total_amount).amount,
    ALTER COLUMN total_vat TYPE bigint USING (total_vat).amount,
    ALTER COLUMN paid_cash SET DEFAULT 0,
    ALTER COLUMN paid_card SET DEFAULT 0,
    ALTER COLUMN total_amount SET DEFAULT 0,
    ALTER COLUMN total_vat SET DEFAULT 0;

-- balance_snapshots
ALTER TABLE balance_snapshots DROP CONSTRAINT balance_snapshots_currency_chk;
ALTER TABLE balance_snapshots ADD COLUMN currency CHAR(3);
UPDATE balance_snapshots SET currency = (amount).currency;
ALTER TABLE balance_snapshots
    ALTER COLUMN amount TYPE bigint USING (amount).amount,
    ALTER COLUMN currency SET NOT NULL;
ALTER TABLE balance_snapshots
    ADD CONSTRAINT balance_snapshots_currency_chk CHECK (currency ~ '^[A-Z]{3}$');

-- scheduled_transactions
ALTER TABLE scheduled_transactions DROP CONSTRAINT scheduled_transactions_amount_chk;
ALTER TABLE scheduled_transactions
    ALTER COLUMN amount TYPE bigint USING (amount).amount,
    ALTER COLUMN to_amount TYPE bigint USING (to_amount).amount;
ALTER TABLE scheduled_transactions
    ADD CONSTRAINT scheduled_transactions_amount_chk CHECK (amount > 0);

-- budgets
ALTER TABLE budgets DROP CONSTRAINT budgets_amount_chk;
ALTER TABLE budgets ALTER COLUMN amount TYPE bigint USING (amount).amount;
ALTER TABLE budgets ADD CONSTRAINT budgets_amount_chk CHECK (amount > 0);

-- transactions
ALTER TABLE transactions DROP CONSTRAINT transactions_amount_chk;
ALTER TABLE transactions DROP CONSTRAINT transactions_currency_chk;
ALTER TABLE transactions DROP CONSTRAINT transactions_to_currency_chk;
ALTER TABLE transactions DROP CONSTRAINT transactions_to_leg_chk;

ALTER TABLE transactions ADD COLUMN currency CHAR(3);
ALTER TABLE transactions ADD COLUMN to_currency CHAR(3);
UPDATE transactions SET currency = (amount).currency, to_currency = (to_amount).currency;
ALTER TABLE transactions
    ALTER COLUMN amount TYPE bigint USING (amount).amount,
    ALTER COLUMN to_amount TYPE bigint USING (to_amount).amount,
    ALTER COLUMN base_amount TYPE bigint USING (base_amount).amount,
    ALTER COLUMN currency SET NOT NULL;

ALTER TABLE transactions
    ADD CONSTRAINT transactions_amount_chk CHECK (amount > 0),
    ADD CONSTRAINT transactions_currency_chk CHECK (currency ~ '^[A-Z]{3}$'),
    ADD CONSTRAINT transactions_to_currency_chk CHECK (to_currency IS NULL OR to_currency ~ '^[A-Z]{3}$'),
    ADD CONSTRAINT transactions_to_leg_chk CHECK (type = 'transfer' OR (to_amount IS NULL AND to_currency IS NULL));

-- accounts
ALTER TABLE accounts
    ALTER COLUMN opening_balance TYPE bigint USING (opening_balance).amount,
    ALTER COLUMN opening_balance SET DEFAULT 0,
    ALTER COLUMN credit_limit TYPE bigint USING (credit_limit).amount,
    ALTER COLUMN principal TYPE bigint USING (principal).amount;

DROP TYPE money_t;
