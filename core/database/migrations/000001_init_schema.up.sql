-- Phase 1 data layer (REQUIREMENTS §4). Money is BIGINT minor units + ISO-4217
-- currency code; never floating point. Domain invariants from §5 are enforced
-- as CHECK constraints where they can be expressed without runtime context.

-- accounts ------------------------------------------------------------------
CREATE TABLE accounts (
    id              UUID PRIMARY KEY     DEFAULT gen_random_uuid(),
    name            TEXT        NOT NULL,
    kind            TEXT        NOT NULL,
    type            TEXT        NOT NULL,
    currency        CHAR(3)     NOT NULL,
    opening_balance BIGINT      NOT NULL DEFAULT 0,
    archived        BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- deposit subtype
    interest_rate   NUMERIC(10, 6),
    term_months     INTEGER,
    maturity_date   DATE,
    capitalization  BOOLEAN,

    -- credit_card subtype
    credit_limit    BIGINT,

    -- loan subtype
    principal       BIGINT,
    start_date      DATE,
    payment_day     INTEGER,

    CONSTRAINT accounts_kind_chk CHECK (kind IN ('asset', 'liability')),
    CONSTRAINT accounts_type_chk CHECK (type IN ('cash', 'debit_card', 'deposit', 'credit_card', 'loan')),
    -- the asset/liability split is fixed by type (§2)
    CONSTRAINT accounts_kind_type_chk CHECK (
        (kind = 'asset' AND type IN ('cash', 'debit_card', 'deposit'))
        OR (kind = 'liability' AND type IN ('credit_card', 'loan'))
    ),
    CONSTRAINT accounts_currency_chk CHECK (currency ~ '^[A-Z]{3}$'),
    CONSTRAINT accounts_payment_day_chk CHECK (payment_day IS NULL OR payment_day BETWEEN 1 AND 31)
);

-- categories ----------------------------------------------------------------
CREATE TABLE categories (
    id         UUID PRIMARY KEY     DEFAULT gen_random_uuid(),
    name       TEXT        NOT NULL,
    parent_id  UUID,
    type       TEXT        NOT NULL,
    icon       TEXT,
    color      TEXT,
    archived   BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT categories_type_chk CHECK (type IN ('expense', 'income')),
    -- needed so the composite FK below can pin a parent to the same tree
    CONSTRAINT categories_id_type_uniq UNIQUE (id, type),
    -- a subcategory must share its parent's type (expense vs income roots, §4)
    CONSTRAINT categories_parent_fk FOREIGN KEY (parent_id, type)
        REFERENCES categories (id, type)
);

-- transactions --------------------------------------------------------------
CREATE TABLE transactions (
    id              UUID PRIMARY KEY     DEFAULT gen_random_uuid(),
    date            TIMESTAMPTZ NOT NULL,
    type            TEXT        NOT NULL,
    from_account_id UUID        REFERENCES accounts (id),
    to_account_id   UUID        REFERENCES accounts (id),
    category_id     UUID        REFERENCES categories (id),
    amount          BIGINT      NOT NULL,
    currency        CHAR(3)     NOT NULL,
    to_amount       BIGINT,
    to_currency     CHAR(3),
    rate_to_base    NUMERIC(20, 10),
    base_amount     BIGINT,
    note            TEXT,
    tags            TEXT[]      NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT transactions_type_chk CHECK (type IN ('expense', 'income', 'transfer')),
    CONSTRAINT transactions_amount_chk CHECK (amount > 0),
    CONSTRAINT transactions_currency_chk CHECK (currency ~ '^[A-Z]{3}$'),
    CONSTRAINT transactions_to_currency_chk CHECK (to_currency IS NULL OR to_currency ~ '^[A-Z]{3}$'),

    -- per-type field applicability (§4) and the transfer/category exclusion (§5)
    CONSTRAINT transactions_shape_chk CHECK (
        (type = 'expense'  AND from_account_id IS NOT NULL AND to_account_id IS NULL     AND category_id IS NOT NULL)
        OR (type = 'income'   AND from_account_id IS NULL     AND to_account_id IS NOT NULL AND category_id IS NOT NULL)
        OR (type = 'transfer' AND from_account_id IS NOT NULL AND to_account_id IS NOT NULL AND category_id IS NULL)
    ),
    -- the second leg exists only for transfers
    CONSTRAINT transactions_to_leg_chk CHECK (
        type = 'transfer' OR (to_amount IS NULL AND to_currency IS NULL)
    ),
    -- a frozen rate and its derived base amount are present together (§3)
    CONSTRAINT transactions_base_chk CHECK ((rate_to_base IS NULL) = (base_amount IS NULL))
);

CREATE INDEX transactions_date_idx ON transactions (date);
CREATE INDEX transactions_from_account_idx ON transactions (from_account_id);
CREATE INDEX transactions_to_account_idx ON transactions (to_account_id);
CREATE INDEX transactions_category_idx ON transactions (category_id);
