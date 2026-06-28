ALTER TABLE scheduled_transactions DROP CONSTRAINT scheduled_transactions_freq_chk;
ALTER TABLE scheduled_transactions
    ADD CONSTRAINT scheduled_transactions_freq_chk
        CHECK (frequency IN ('daily', 'weekly', 'monthly', 'yearly'));
