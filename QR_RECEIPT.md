# QR Receipt Feature — Requirements

## Overview

Users scan Uzbekistan fiscal QR codes (ofd.soliq.uz) from paper receipts, optionally photograph
the paper, and the system saves full parsed receipt data with photo to the PFM backend.

---

## Components

### 1. UZ Proxy Service

Single PHP script (`proxy.php`), deployed on UZ VPS. Accepts `POST /fetch` with URL in body and bearer token in header, returns raw HTML. See `proxy.php`.

---

### 2. Go Backend — Receipt Ingestion

**Endpoint:**

```
POST /api/v1/receipts
Authorization: Bearer <user_jwt>
Content-Type: multipart/form-data

Fields:
  qr_url   string   required   Full ofd.soliq.uz check URL
  photo    file     required   JPEG image of the paper receipt
```

**Response (immediate):**
```json
{
  "id": "uuid",
  "status": "pending"
}
```

**Async processing pipeline (background goroutine):**

1. Upload photo to R2 at key `receipts/{year}/{month}/{receipt_id}.jpg`
2. Call UZ proxy → POST `/fetch` with `qr_url`
3. On proxy success: set status = `html_received`, store `raw_html`
4. Parse HTML with goquery (see Parsing section)
5. On parse success: set status = `success`, store all parsed fields
6. On any failure: set status = `failed`, store error reason

**Get receipt:**
```
GET /api/v1/receipts/:id
Authorization: Bearer <user_jwt>
```
Returns full receipt with items. Client polls until status is `success` or `failed`.

**List receipts:**
```
GET /api/v1/receipts?page=1&limit=20
Authorization: Bearer <user_jwt>
```
Returns paginated list of receipt headers (no items).

---

### 2. HTML Parsing (goquery)

Library: `github.com/PuerkitoBio/goquery`

#### Receipt Header Fields

| Field | Selector / Strategy |
|-------|-------------------|
| `receipt_type` | `h3` first occurrence, trim whitespace |
| `merchant_name` | `h3[style*="font-weight: bold"]` text |
| `merchant_address` | text node between merchant `h3` and following `i` tag |
| `merchant_tin` | `i` tag immediately after address text |
| `terminal_id` | first standalone `b` tag after merchant block (or `t=` from QR URL params) |
| `receipt_seq` | `span.left b` where parent span text contains "Chek raqami" |
| `device_name` | `span.left b` where parent span text contains "Onlayn NKM nomi" |
| `serial_number` | `span.left b` where parent span text contains "SN" |
| `received_at` | `i` tag containing date pattern `DD.MM.YYYY, HH:MM` |

#### Payment & Totals Fields

Iterate all `tr` elements, match by first `td` text content:

| Label text | Field |
|-----------|-------|
| `Naqd pul` | `paid_cash` |
| `Bank kartalari` | `paid_card` |
| `Bank kartasi turi` | `card_type` |
| `Jami to'lov:` | `total_amount` |
| `Umumiy QQS qiymati` | `total_vat` |

#### Amount Parsing

All monetary values displayed as `16,480.00` (comma thousands, dot decimal, always 2 decimal places).
Use `go-money` (`github.com/rhymond/go-money`). Parse as integer without floats:

```go
func parseAmount(s string) (*money.Money, error) {
    s = strings.ReplaceAll(s, ",", "")  // "16480.00"
    s = strings.ReplaceAll(s, ".", "")  // "1648000"
    s = strings.TrimSpace(s)
    amount, err := strconv.ParseInt(s, 10, 64)
    if err != nil {
        return nil, err
    }
    return money.New(amount, "UZS"), nil
}
```

Store `money.Money.Amount()` (int64, tiyin) in DB.

#### Line Items

Each item is a group of rows starting with `tr.products-row`:

| Field | Source |
|-------|--------|
| `name` | `.products-row td:eq(0)` text, trimmed |
| `quantity` | `.products-row td:eq(1)` text, trimmed |
| `price` | `.price-sum` text → parseAmount |
| `vat_amount` | first `.nds-sum` text → parseAmount |
| `vat_rate` | second `.nds-sum` text, strip `%`, parse int |
| `discount` | `Chegirma/Boshqa` row, left of `/` |
| `other` | `Chegirma/Boshqa` row, right of `/` |
| `barcode` | `Shtrix kodi` row value |
| `ikpu_code` | `MXIK kodi` row value |
| `ikpu_name` | `MXIK nomi` row value |
| `unit` | `O'lchov birligi` row value |
| `marking_code` | `Markirovka kodi` row value (nullable) |
| `consignor_tin` | `Komitent STIR/JSHSHIR` row value (nullable) |

#### Merchant Coordinates

Extract from inline `<script>` tag using regexp:

```go
var placemarkRe = regexp.MustCompile(
    `new ymaps\.Placemark\(\[([0-9.]+),\s*([0-9.]+)\]`,
)
```

Store as strings into `NUMERIC(10,8)` / `NUMERIC(11,8)` DB columns. No float arithmetic.

---

## Data Model

### `receipts`

| Column | Type | Notes |
|--------|------|-------|
| `id` | UUID PK | |
| `qr_url` | TEXT NOT NULL | full ofd.soliq.uz URL |
| `terminal_id` | TEXT | `t=` param |
| `receipt_seq` | INTEGER | `r=` param |
| `fiscal_sign` | TEXT | `s=` param |
| `received_at` | TIMESTAMPTZ | parsed from `c=` param or HTML |
| `receipt_type` | TEXT | e.g. `Savdo cheki/Sotuv` |
| `merchant_name` | TEXT | |
| `merchant_tin` | TEXT | |
| `merchant_address` | TEXT | |
| `device_name` | TEXT | |
| `serial_number` | TEXT | |
| `paid_cash` | BIGINT | tiyin |
| `paid_card` | BIGINT | tiyin |
| `card_type` | TEXT | e.g. `Shaxsiy` |
| `total_amount` | BIGINT | tiyin |
| `total_vat` | BIGINT | tiyin |
| `merchant_lat` | NUMERIC(10,8) | from Yandex Placemark JS |
| `merchant_lng` | NUMERIC(11,8) | from Yandex Placemark JS |
| `photo_key` | TEXT | R2 key: `receipts/{year}/{month}/{id}.jpg` |
| `raw_html` | TEXT | cached scraped HTML |
| `status` | TEXT | `pending` / `html_received` / `success` / `failed` |
| `error` | TEXT | failure reason if status=failed |
| `scraped_at` | TIMESTAMPTZ | |
| `created_at` | TIMESTAMPTZ | |

### `receipt_items`

| Column | Type | Notes |
|--------|------|-------|
| `id` | UUID PK | |
| `receipt_id` | UUID FK → receipts | |
| `name` | TEXT | |
| `quantity` | TEXT | raw string, e.g. `1` |
| `price` | BIGINT | tiyin |
| `vat_amount` | BIGINT | tiyin |
| `vat_rate` | INTEGER | percent |
| `discount` | BIGINT | tiyin |
| `other` | BIGINT | tiyin |
| `barcode` | TEXT | nullable |
| `ikpu_code` | TEXT | nullable |
| `ikpu_name` | TEXT | nullable |
| `unit` | TEXT | nullable |
| `marking_code` | TEXT | nullable |
| `consignor_tin` | TEXT | nullable |

---

## Dependencies

### UZ Proxy
- Go stdlib `net/http`, `net/http/cookiejar`

### core (additions)
- `github.com/PuerkitoBio/goquery` — HTML parsing
- `github.com/rhymond/go-money` — already in project
- Cloudflare R2 SDK (aws-sdk-go-v2 with R2 endpoint) — if not already present

---

## Environment Variables

### UZ Proxy
| Variable | Description |
|----------|-------------|
| `PROXY_SECRET` | Bearer token shared with core |
| `PORT` | HTTP listen port |

### core (additions)
| Variable | Description |
|----------|-------------|
| `PROXY_URL` | Base URL of UZ proxy, e.g. `http://uz-vps:8080` |
| `PROXY_SECRET` | Bearer token for UZ proxy |
| `R2_BUCKET` | Cloudflare R2 bucket name |
| `R2_ACCOUNT_ID` | Cloudflare account ID |
| `R2_ACCESS_KEY_ID` | R2 access key |
| `R2_SECRET_ACCESS_KEY` | R2 secret key |