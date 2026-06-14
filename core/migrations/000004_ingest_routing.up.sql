-- Ingest routing lives in the app (system of record), not the bot: accounts
-- carry the card last4 that identifies them, categories can be marked as the
-- default ingest bucket, and merchant strings route to a category via rules.

ALTER TABLE accounts ADD COLUMN card_last4 TEXT;
CREATE UNIQUE INDEX accounts_card_last4_uniq
    ON accounts (card_last4) WHERE card_last4 IS NOT NULL;

-- system_key tags the built-in "Uncategorized" buckets so ingest can find them.
ALTER TABLE categories ADD COLUMN system_key TEXT;
CREATE UNIQUE INDEX categories_system_key_uniq
    ON categories (system_key) WHERE system_key IS NOT NULL;

-- merchant -> category rules; pattern matched as a case-insensitive substring.
CREATE TABLE category_rules (
    id          UUID PRIMARY KEY     DEFAULT gen_random_uuid(),
    pattern     TEXT        NOT NULL,
    category_id UUID        NOT NULL REFERENCES categories (id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
