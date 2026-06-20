-- Let an account opt out of net-worth aggregation (e.g. a tracking-only or
-- shared account) while still recording its transactions. Default TRUE keeps
-- every existing account in net worth.
ALTER TABLE accounts
    ADD COLUMN include_in_net_worth BOOLEAN NOT NULL DEFAULT TRUE;
