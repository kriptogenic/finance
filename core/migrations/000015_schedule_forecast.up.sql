-- Scheduled transactions are no longer auto-materialized by a worker; they now
-- describe expected recurring cash flow that feeds the forecast. next_run was
-- the next occurrence the worker would post; it becomes start_date, a static
-- recurrence anchor. last_run_at tracked the worker and is dropped.
ALTER INDEX scheduled_transactions_next_run_idx RENAME TO scheduled_transactions_start_date_idx;
ALTER TABLE scheduled_transactions RENAME COLUMN next_run TO start_date;
ALTER TABLE scheduled_transactions DROP COLUMN last_run_at;
