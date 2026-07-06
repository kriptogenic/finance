DELETE FROM receipts
WHERE id IN (
    SELECT id FROM (
        SELECT id, row_number() OVER (
            PARTITION BY terminal_id, receipt_seq, fiscal_sign
            ORDER BY (transaction_id IS NOT NULL) DESC, created_at ASC, id ASC
        ) AS rn
        FROM receipts
        WHERE terminal_id IS NOT NULL AND receipt_seq IS NOT NULL AND fiscal_sign IS NOT NULL
    ) d
    WHERE d.rn > 1
);

CREATE UNIQUE INDEX receipts_fiscal_uq
    ON receipts (terminal_id, receipt_seq, fiscal_sign)
    WHERE terminal_id IS NOT NULL AND receipt_seq IS NOT NULL AND fiscal_sign IS NOT NULL;
