-- soliq switched from a scraped HTML page to a signed JSON API; the stored raw
-- response is now JSON, not HTML.
ALTER TABLE receipts RENAME COLUMN raw_html TO raw_payload;
