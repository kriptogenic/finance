-- Switch id generation from random v4 to time-ordered UUIDv7 so new ids sort by
-- creation time (better index locality). Postgres 18 ships a native uuidv7();
-- on older servers (our PG16 dev/test containers) define the canonical SQL
-- implementation in public so the DEFAULT resolves on every version. Unqualified
-- uuidv7() resolves to the native (pg_catalog) function first when present.
DO $do$
BEGIN
    IF to_regprocedure('pg_catalog.uuidv7()') IS NULL THEN
        CREATE FUNCTION public.uuidv7() RETURNS uuid
        AS $fn$
            SELECT encode(
                set_bit(
                    set_bit(
                        overlay(uuid_send(gen_random_uuid())
                                PLACING substring(int8send(floor(extract(epoch FROM clock_timestamp()) * 1000)::bigint) FROM 3)
                                FROM 1 FOR 6),
                        52, 1),
                    53, 1),
                'hex')::uuid;
        $fn$ LANGUAGE sql VOLATILE;
    END IF;
END
$do$;

ALTER TABLE accounts               ALTER COLUMN id SET DEFAULT uuidv7();
ALTER TABLE categories             ALTER COLUMN id SET DEFAULT uuidv7();
ALTER TABLE transactions           ALTER COLUMN id SET DEFAULT uuidv7();
ALTER TABLE budgets                ALTER COLUMN id SET DEFAULT uuidv7();
ALTER TABLE category_rules         ALTER COLUMN id SET DEFAULT uuidv7();
ALTER TABLE scheduled_transactions ALTER COLUMN id SET DEFAULT uuidv7();
ALTER TABLE receipts               ALTER COLUMN id SET DEFAULT uuidv7();
ALTER TABLE receipt_items          ALTER COLUMN id SET DEFAULT uuidv7();
ALTER TABLE audit_reports          ALTER COLUMN id SET DEFAULT uuidv7();
ALTER TABLE loan_schedules         ALTER COLUMN id SET DEFAULT uuidv7();
