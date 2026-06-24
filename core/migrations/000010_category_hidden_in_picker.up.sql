-- Let a category be hidden from the categorize picker (e.g. an ingest-only
-- bucket) while still usable by rules and shown on existing transactions.
ALTER TABLE categories
    ADD COLUMN hidden_in_picker BOOLEAN NOT NULL DEFAULT FALSE;
