-- Link a fiscal receipt to the expense transaction it documents (1:1). The
-- receipt keeps the FK; deleting the transaction just clears the link.
ALTER TABLE receipts
    ADD COLUMN transaction_id UUID REFERENCES transactions (id) ON DELETE SET NULL;

CREATE UNIQUE INDEX receipts_transaction_id_uq
    ON receipts (transaction_id) WHERE transaction_id IS NOT NULL;
