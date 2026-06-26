-- Fiscal QR receipts scanned from paper: the QR url + stored photo, plus the
-- header, totals, line items and merchant coordinates scraped+parsed from
-- ofd.soliq.uz. Monetary columns are integer minor units (tiyin) in UZS.
CREATE TABLE receipts (
    id               UUID PRIMARY KEY     DEFAULT gen_random_uuid(),
    qr_url           TEXT        NOT NULL,
    status           TEXT        NOT NULL DEFAULT 'pending',
    error            TEXT,

    terminal_id      TEXT,
    receipt_seq      INTEGER,
    fiscal_sign      TEXT,
    received_at      TIMESTAMPTZ,

    receipt_type     TEXT,
    merchant_name    TEXT,
    merchant_tin     TEXT,
    merchant_address TEXT,
    device_name      TEXT,
    serial_number    TEXT,
    card_type        TEXT,
    merchant_lat     NUMERIC(10, 8),
    merchant_lng     NUMERIC(11, 8),

    paid_cash        BIGINT      NOT NULL DEFAULT 0,
    paid_card        BIGINT      NOT NULL DEFAULT 0,
    total_amount     BIGINT      NOT NULL DEFAULT 0,
    total_vat        BIGINT      NOT NULL DEFAULT 0,

    photo_key        TEXT,
    raw_html         TEXT,
    scraped_at       TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT receipts_status_chk CHECK (status IN ('pending', 'html_received', 'success', 'failed'))
);

CREATE INDEX receipts_created_at_idx ON receipts (created_at DESC);

CREATE TABLE receipt_items (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    receipt_id    UUID    NOT NULL REFERENCES receipts (id) ON DELETE CASCADE,
    name          TEXT    NOT NULL,
    quantity      TEXT    NOT NULL DEFAULT '',
    price         BIGINT  NOT NULL DEFAULT 0,
    vat_amount    BIGINT  NOT NULL DEFAULT 0,
    vat_rate      INTEGER NOT NULL DEFAULT 0,
    discount      BIGINT  NOT NULL DEFAULT 0,
    other         BIGINT  NOT NULL DEFAULT 0,
    barcode       TEXT,
    ikpu_code     TEXT,
    ikpu_name     TEXT,
    unit          TEXT,
    marking_code  TEXT,
    consignor_tin TEXT
);

CREATE INDEX receipt_items_receipt_id_idx ON receipt_items (receipt_id);
