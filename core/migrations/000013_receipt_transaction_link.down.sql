DROP INDEX IF EXISTS receipts_transaction_id_uq;
ALTER TABLE receipts DROP COLUMN IF EXISTS transaction_id;
