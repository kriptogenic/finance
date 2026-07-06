ALTER TABLE accounts               ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE categories             ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE transactions           ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE budgets                ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE category_rules         ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE scheduled_transactions ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE receipts               ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE receipt_items          ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE audit_reports          ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE loan_schedules         ALTER COLUMN id SET DEFAULT gen_random_uuid();

-- Only our public copy (PG16); never the native pg_catalog.uuidv7() on PG18.
DROP FUNCTION IF EXISTS public.uuidv7();
