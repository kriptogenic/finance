-- External ingest support (e.g. the Telegram bank-notification userbot).
-- external_id is a stable per-source key used for idempotent upserts; partial
-- unique so normal UI transactions (NULL external_id) never collide.
-- transfer_group_id ties the two legs of a paired transfer for traceability.
ALTER TABLE transactions
    ADD COLUMN external_id       TEXT,
    ADD COLUMN transfer_group_id TEXT;

CREATE UNIQUE INDEX transactions_external_id_uniq
    ON transactions (external_id)
    WHERE external_id IS NOT NULL;
