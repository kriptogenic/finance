-- Public holidays that push a loan payment forward to the next business day.
CREATE TABLE holidays (
    day  DATE PRIMARY KEY,
    name TEXT NOT NULL
);

-- Persisted, editable amortization plan for a loan account. Seeded from the
-- annuity generator, then rows absorb real payment dates (weekend/holiday roll
-- or a manual override) and the interest/principal split those dates imply.
CREATE TABLE loan_schedules (
    id            UUID PRIMARY KEY     DEFAULT gen_random_uuid(),
    account_id    UUID        NOT NULL REFERENCES accounts (id),
    period        INT         NOT NULL,
    due_date      DATE        NOT NULL, -- effective date: nominal, rolled, or overridden
    date_override DATE,                 -- manual edge-case date; when set, wins over the calendar
    payment       money_t     NOT NULL, -- = principal + interest, in the loan's currency
    principal     money_t     NOT NULL,
    interest      money_t     NOT NULL,
    balance       money_t     NOT NULL, -- remaining principal after this installment
    paid          BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT loan_schedules_period_uq UNIQUE (account_id, period),
    CONSTRAINT loan_schedules_period_chk CHECK (period > 0),
    CONSTRAINT loan_schedules_payment_chk CHECK ((payment).amount >= 0),
    CONSTRAINT loan_schedules_split_chk CHECK ((payment).amount = (principal).amount + (interest).amount)
);

CREATE INDEX loan_schedules_account_idx ON loan_schedules (account_id, period);
