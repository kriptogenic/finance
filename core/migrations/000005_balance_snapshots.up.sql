CREATE TABLE balance_snapshots (
    card_last4  TEXT PRIMARY KEY,
    bank        TEXT,
    amount      BIGINT      NOT NULL,
    currency    CHAR(3)     NOT NULL,
    source      TEXT,
    reported_at TIMESTAMPTZ NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT balance_snapshots_currency_chk CHECK (currency ~ '^[A-Z]{3}$')
);
