-- One-off backfill: rewrite existing v4 primary keys as time-ordered UUIDv7,
-- keeping every foreign-key reference intact. Each new id encodes the row's
-- created_at, so historical rows sort by creation time just like new ones.
--
-- Run ONCE, against a backup-verified database. Idempotency: safe to re-run —
-- rows already carrying a v7 id (version nibble 7) are skipped.
--
-- Strategy: snapshot all FK definitions, drop them, remap parents + children via
-- old->new maps, then recreate the FKs. Whole thing is one transaction, so it
-- either fully succeeds or rolls back.

BEGIN;

-- Build a v7 uuid stamped at an arbitrary time (works on PG16 and PG18). The
-- high 48 bits are the millisecond timestamp; the rest stays random.
CREATE FUNCTION pg_temp.uuidv7_at(ts timestamptz) RETURNS uuid
LANGUAGE sql VOLATILE AS $$
    SELECT encode(
        set_bit(
            set_bit(
                overlay(uuid_send(gen_random_uuid())
                        PLACING substring(int8send(floor(extract(epoch FROM ts) * 1000)::bigint) FROM 3)
                        FROM 1 FOR 6),
                52, 1),
            53, 1),
        'hex')::uuid;
$$;

-- A row already on v7 has '7' as the version nibble (first char of 3rd group).
CREATE FUNCTION pg_temp.is_v7(u uuid) RETURNS boolean
LANGUAGE sql IMMUTABLE AS $$
    SELECT substring(u::text FROM 15 FOR 1) = '7';
$$;

-- 1. Snapshot every foreign key, then drop them all so PKs can be rewritten.
CREATE TEMP TABLE fk_backup ON COMMIT DROP AS
SELECT conrelid::regclass::text AS tbl, conname, pg_get_constraintdef(oid) AS def
FROM pg_constraint
WHERE contype = 'f';

DO $$
DECLARE r record;
BEGIN
    FOR r IN SELECT tbl, conname FROM fk_backup LOOP
        EXECUTE format('ALTER TABLE %s DROP CONSTRAINT %I', r.tbl, r.conname);
    END LOOP;
END $$;

-- 2. Build old->new maps. Only rows still on v4 get a new id (idempotent).
--    Maps are built from current (old) ids before any update runs.
CREATE TEMP TABLE map_accounts ON COMMIT DROP AS
    SELECT id AS old, pg_temp.uuidv7_at(created_at) AS new FROM accounts WHERE NOT pg_temp.is_v7(id);
CREATE TEMP TABLE map_categories ON COMMIT DROP AS
    SELECT id AS old, pg_temp.uuidv7_at(created_at) AS new FROM categories WHERE NOT pg_temp.is_v7(id);
CREATE TEMP TABLE map_transactions ON COMMIT DROP AS
    SELECT id AS old, pg_temp.uuidv7_at(created_at) AS new FROM transactions WHERE NOT pg_temp.is_v7(id);
CREATE TEMP TABLE map_receipts ON COMMIT DROP AS
    SELECT id AS old, pg_temp.uuidv7_at(created_at) AS new FROM receipts WHERE NOT pg_temp.is_v7(id);
CREATE TEMP TABLE map_budgets ON COMMIT DROP AS
    SELECT id AS old, pg_temp.uuidv7_at(created_at) AS new FROM budgets WHERE NOT pg_temp.is_v7(id);
CREATE TEMP TABLE map_category_rules ON COMMIT DROP AS
    SELECT id AS old, pg_temp.uuidv7_at(created_at) AS new FROM category_rules WHERE NOT pg_temp.is_v7(id);
CREATE TEMP TABLE map_scheduled_transactions ON COMMIT DROP AS
    SELECT id AS old, pg_temp.uuidv7_at(created_at) AS new FROM scheduled_transactions WHERE NOT pg_temp.is_v7(id);
CREATE TEMP TABLE map_audit_reports ON COMMIT DROP AS
    SELECT id AS old, pg_temp.uuidv7_at(created_at) AS new FROM audit_reports WHERE NOT pg_temp.is_v7(id);
CREATE TEMP TABLE map_loan_schedules ON COMMIT DROP AS
    SELECT id AS old, pg_temp.uuidv7_at(created_at) AS new FROM loan_schedules WHERE NOT pg_temp.is_v7(id);
-- receipt_items has no created_at; borrow the parent receipt's time for ordering.
CREATE TEMP TABLE map_receipt_items ON COMMIT DROP AS
    SELECT ri.id AS old, pg_temp.uuidv7_at(r.created_at) AS new
    FROM receipt_items ri JOIN receipts r ON r.id = ri.receipt_id
    WHERE NOT pg_temp.is_v7(ri.id);

-- 3. Rewrite children (FK columns) and then each parent PK. New v7 ids are
--    disjoint from old v4 ids, so matching on `old` stays unambiguous even after
--    some columns have been updated.

-- accounts
UPDATE transactions t           SET from_account_id = m.new FROM map_accounts m WHERE t.from_account_id = m.old;
UPDATE transactions t           SET to_account_id   = m.new FROM map_accounts m WHERE t.to_account_id   = m.old;
UPDATE scheduled_transactions s SET from_account_id = m.new FROM map_accounts m WHERE s.from_account_id = m.old;
UPDATE scheduled_transactions s SET to_account_id   = m.new FROM map_accounts m WHERE s.to_account_id   = m.old;
UPDATE loan_schedules l         SET account_id      = m.new FROM map_accounts m WHERE l.account_id      = m.old;
UPDATE accounts a               SET id              = m.new FROM map_accounts m WHERE a.id              = m.old;

-- categories (incl. self parent_id)
UPDATE transactions t           SET category_id = m.new FROM map_categories m WHERE t.category_id = m.old;
UPDATE budgets b                SET category_id = m.new FROM map_categories m WHERE b.category_id = m.old;
UPDATE category_rules c         SET category_id = m.new FROM map_categories m WHERE c.category_id = m.old;
UPDATE scheduled_transactions s SET category_id = m.new FROM map_categories m WHERE s.category_id = m.old;
UPDATE categories c             SET parent_id   = m.new FROM map_categories m WHERE c.parent_id   = m.old;
UPDATE categories c             SET id          = m.new FROM map_categories m WHERE c.id          = m.old;

-- transactions
UPDATE receipts r      SET transaction_id = m.new FROM map_transactions m WHERE r.transaction_id = m.old;
UPDATE consumed_legs l SET transaction_id = m.new FROM map_transactions m WHERE l.transaction_id = m.old;
UPDATE transactions t  SET id             = m.new FROM map_transactions m WHERE t.id             = m.old;

-- receipts
UPDATE receipt_items ri SET receipt_id = m.new FROM map_receipts m WHERE ri.receipt_id = m.old;
UPDATE receipts r       SET id         = m.new FROM map_receipts m WHERE r.id          = m.old;

-- leaf tables (no inbound references)
UPDATE receipt_items ri         SET id = m.new FROM map_receipt_items m         WHERE ri.id = m.old;
UPDATE budgets b                SET id = m.new FROM map_budgets m               WHERE b.id  = m.old;
UPDATE category_rules c         SET id = m.new FROM map_category_rules m        WHERE c.id  = m.old;
UPDATE scheduled_transactions s SET id = m.new FROM map_scheduled_transactions m WHERE s.id = m.old;
UPDATE audit_reports a          SET id = m.new FROM map_audit_reports m         WHERE a.id  = m.old;
UPDATE loan_schedules l         SET id = m.new FROM map_loan_schedules m        WHERE l.id  = m.old;

-- 4. Recreate every foreign key from its saved definition (validates the data).
DO $$
DECLARE r record;
BEGIN
    FOR r IN SELECT tbl, conname, def FROM fk_backup LOOP
        EXECUTE format('ALTER TABLE %s ADD CONSTRAINT %I %s', r.tbl, r.conname, r.def);
    END LOOP;
END $$;

-- 5. Sanity check: every id must now be v7. Raises if any slipped through.
DO $$
DECLARE bad int;
BEGIN
    SELECT count(*) INTO bad FROM (
        SELECT id FROM accounts UNION ALL SELECT id FROM categories UNION ALL
        SELECT id FROM transactions UNION ALL SELECT id FROM receipts UNION ALL
        SELECT id FROM receipt_items UNION ALL SELECT id FROM budgets UNION ALL
        SELECT id FROM category_rules UNION ALL SELECT id FROM scheduled_transactions UNION ALL
        SELECT id FROM audit_reports UNION ALL SELECT id FROM loan_schedules
    ) x WHERE substring(id::text FROM 15 FOR 1) <> '7';
    IF bad > 0 THEN
        RAISE EXCEPTION 'backfill incomplete: % non-v7 ids remain', bad;
    END IF;
END $$;

COMMIT;
