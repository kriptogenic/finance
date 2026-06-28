-- Allow one-time scheduled transactions (e.g. a birthday present): a single
-- occurrence on start_date, no recurrence.
ALTER TABLE scheduled_transactions DROP CONSTRAINT scheduled_transactions_freq_chk;
ALTER TABLE scheduled_transactions
    ADD CONSTRAINT scheduled_transactions_freq_chk
        CHECK (frequency IN ('once', 'daily', 'weekly', 'monthly', 'yearly'));
