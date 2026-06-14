DROP INDEX IF EXISTS transactions_external_id_uniq;
ALTER TABLE transactions
    DROP COLUMN IF EXISTS external_id,
    DROP COLUMN IF EXISTS transfer_group_id;
