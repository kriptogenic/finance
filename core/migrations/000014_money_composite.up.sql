-- Collapse every (amount, currency) pair into a single composite-typed column so
-- the currency always travels with the amount. Columns with an implied currency
-- (base_amount, budgets, scheduled, receipts) are backfilled: base/UZS where the
-- value is in base currency, the account currency for account-scoped amounts.
-- DROP first so `migrate fresh` (which drops tables but not types) can re-apply.
DROP TYPE IF EXISTS money_t;
CREATE TYPE money_t AS (amount bigint, currency text);

-- accounts: opening_balance/credit_limit/principal use the account's own currency
-- (kept as a plain column — it is the account's currency identity).
ALTER TABLE accounts
    ALTER COLUMN opening_balance DROP DEFAULT,
    ALTER COLUMN opening_balance TYPE money_t USING ROW(opening_balance, currency)::money_t,
    ALTER COLUMN credit_limit TYPE money_t
        USING (CASE WHEN credit_limit IS NULL THEN NULL ELSE ROW(credit_limit, currency)::money_t END),
    ALTER COLUMN principal TYPE money_t
        USING (CASE WHEN principal IS NULL THEN NULL ELSE ROW(principal, currency)::money_t END);

-- transactions
ALTER TABLE transactions DROP CONSTRAINT transactions_amount_chk;
ALTER TABLE transactions DROP CONSTRAINT transactions_currency_chk;
ALTER TABLE transactions DROP CONSTRAINT transactions_to_currency_chk;
ALTER TABLE transactions DROP CONSTRAINT transactions_to_leg_chk;

ALTER TABLE transactions
    ALTER COLUMN amount TYPE money_t USING ROW(amount, currency)::money_t,
    ALTER COLUMN to_amount TYPE money_t
        USING (CASE WHEN to_amount IS NULL THEN NULL ELSE ROW(to_amount, to_currency)::money_t END),
    ALTER COLUMN base_amount TYPE money_t
        USING (CASE WHEN base_amount IS NULL THEN NULL ELSE ROW(base_amount, 'UZS')::money_t END);

ALTER TABLE transactions DROP COLUMN currency;
ALTER TABLE transactions DROP COLUMN to_currency;

ALTER TABLE transactions
    ADD CONSTRAINT transactions_amount_chk CHECK ((amount).amount > 0),
    ADD CONSTRAINT transactions_currency_chk CHECK ((amount).currency ~ '^[A-Z]{3}$'),
    ADD CONSTRAINT transactions_to_currency_chk CHECK (to_amount IS NULL OR (to_amount).currency ~ '^[A-Z]{3}$'),
    ADD CONSTRAINT transactions_to_leg_chk CHECK (type = 'transfer' OR to_amount IS NULL);

-- budgets: amount is in base currency
ALTER TABLE budgets DROP CONSTRAINT budgets_amount_chk;
ALTER TABLE budgets
    ALTER COLUMN amount TYPE money_t USING ROW(amount, 'UZS')::money_t;
ALTER TABLE budgets
    ADD CONSTRAINT budgets_amount_chk CHECK ((amount).amount > 0);

-- scheduled_transactions: no currency column — derive it from the account the
-- amount belongs to (from for expense/transfer, to for income). A subquery is
-- not allowed in a USING transform, so stage the currency in temp columns first.
ALTER TABLE scheduled_transactions DROP CONSTRAINT scheduled_transactions_amount_chk;
ALTER TABLE scheduled_transactions ADD COLUMN _cur text, ADD COLUMN _to_cur text;
UPDATE scheduled_transactions s SET _cur = a.currency
    FROM accounts a WHERE a.id = COALESCE(s.from_account_id, s.to_account_id);
UPDATE scheduled_transactions s SET _to_cur = a.currency
    FROM accounts a WHERE a.id = s.to_account_id;
ALTER TABLE scheduled_transactions
    ALTER COLUMN amount TYPE money_t USING ROW(amount, _cur)::money_t,
    ALTER COLUMN to_amount TYPE money_t
        USING (CASE WHEN to_amount IS NULL THEN NULL ELSE ROW(to_amount, _to_cur)::money_t END);
ALTER TABLE scheduled_transactions DROP COLUMN _cur, DROP COLUMN _to_cur;
ALTER TABLE scheduled_transactions
    ADD CONSTRAINT scheduled_transactions_amount_chk CHECK ((amount).amount > 0);

-- balance_snapshots
ALTER TABLE balance_snapshots DROP CONSTRAINT balance_snapshots_currency_chk;
ALTER TABLE balance_snapshots
    ALTER COLUMN amount TYPE money_t USING ROW(amount, currency)::money_t;
ALTER TABLE balance_snapshots DROP COLUMN currency;
ALTER TABLE balance_snapshots
    ADD CONSTRAINT balance_snapshots_currency_chk CHECK ((amount).currency ~ '^[A-Z]{3}$');

-- receipts + items: always UZS (tiyin)
ALTER TABLE receipts
    ALTER COLUMN paid_cash DROP DEFAULT,
    ALTER COLUMN paid_card DROP DEFAULT,
    ALTER COLUMN total_amount DROP DEFAULT,
    ALTER COLUMN total_vat DROP DEFAULT,
    ALTER COLUMN paid_cash TYPE money_t USING ROW(paid_cash, 'UZS')::money_t,
    ALTER COLUMN paid_card TYPE money_t USING ROW(paid_card, 'UZS')::money_t,
    ALTER COLUMN total_amount TYPE money_t USING ROW(total_amount, 'UZS')::money_t,
    ALTER COLUMN total_vat TYPE money_t USING ROW(total_vat, 'UZS')::money_t,
    ALTER COLUMN paid_cash SET DEFAULT ROW(0, 'UZS')::money_t,
    ALTER COLUMN paid_card SET DEFAULT ROW(0, 'UZS')::money_t,
    ALTER COLUMN total_amount SET DEFAULT ROW(0, 'UZS')::money_t,
    ALTER COLUMN total_vat SET DEFAULT ROW(0, 'UZS')::money_t;

ALTER TABLE receipt_items
    ALTER COLUMN price DROP DEFAULT,
    ALTER COLUMN vat_amount DROP DEFAULT,
    ALTER COLUMN discount DROP DEFAULT,
    ALTER COLUMN other DROP DEFAULT,
    ALTER COLUMN price TYPE money_t USING ROW(price, 'UZS')::money_t,
    ALTER COLUMN vat_amount TYPE money_t USING ROW(vat_amount, 'UZS')::money_t,
    ALTER COLUMN discount TYPE money_t USING ROW(discount, 'UZS')::money_t,
    ALTER COLUMN other TYPE money_t USING ROW(other, 'UZS')::money_t,
    ALTER COLUMN price SET DEFAULT ROW(0, 'UZS')::money_t,
    ALTER COLUMN vat_amount SET DEFAULT ROW(0, 'UZS')::money_t,
    ALTER COLUMN discount SET DEFAULT ROW(0, 'UZS')::money_t,
    ALTER COLUMN other SET DEFAULT ROW(0, 'UZS')::money_t;
