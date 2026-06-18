# bank-sms-reader — Requirements

## Purpose

An Android background service application that monitors incoming SMS messages from Infinbank, parses transaction notifications into structured data, and forwards them to the personal finance manager app via its ingest API.

---

## Scope

- **In scope:** SMS transaction parsing, structured forwarding, local retry on failure, parse failure logging
- **Out of scope:** Push notification handling (deferred), UI beyond minimal status screen, multi-bank support (deferred), card-to-card transfer collapse
- **Future:** Push notification listener may be added as a second source; architecture must not preclude this

---

## Source: SMS

### Sender filter

- Sender address: `infinbank` (case-insensitive match)
- Any SMS not from this sender is ignored entirely and never logged or stored

### Known card

- Card number (masked): `404800***7476`
- All transactions from this sender belong to this card
- Card-to-account mapping is a configuration value, not hardcoded

### Message format

All transaction messages follow this template:

```
Pokupka: {merchant} {city} {country} {datetime}, karta {masked_card}. summa: {amount} {currency}, balans: {balance} {currency}
```

**Field details:**

| Field | Format | Example |
|---|---|---|
| `merchant` | Uppercase string, may contain dots and spaces | `YANDEX.GO YUNUSOBOD TUM` |
| `city` | Uppercase string | `Tashkent`, `YUNUSOBOD TUM` |
| `country` | 3-letter ISO code | `UZB`, `USA` |
| `datetime` | `YYYY-MM-DD HH:MM:SS` | `2026-06-18 10:08:41` |
| `masked_card` | `NNNNNN***NNNN` | `404800***7476` |
| `amount` | European formatting: space thousands separator, comma decimal | `28 500.00` |
| `balance` | Same formatting as amount | `19 315 740.00` |
| `currency` | ISO 4217 uppercase | `UZS`, `USD` |

**Real examples:**

```
Pokupka: YANDEX.GO YUNUSOBOD TUM UZB 2026-06-18 10:08:41, karta 404800***7476. summa: 28 500.00 UZS, balans: 19 315 740.00 UZS
Pokupka: ANTHROPIC +14152360599 USA 2026-06-17 07:39:15, karta 404800***7476. summa: 5.60 USD, balans: 19 344 240.00 UZS
Pokupka: HUMO MONEY TRANSFER Tashkent UZB 2026-06-17 10:33:25, karta 404800***7476. summa: 550 000.00 UZS, balans: 19 412 000.00 UZS
```

---

## Parsing rules

### Transaction type

- All messages beginning with `Pokupka:` are classified as **expense** transactions
- No other transaction types are handled in this version
- Messages not matching any known prefix are dropped and logged as parse failures

### Amount parsing

- Strip space thousand separators
- Comma is the decimal separator — replace `,` with `.` before parsing
- Convert to **integer minor units** (multiply by 100, never use floating point)
- Examples: `28 500.00 UZS` → `2850000`, `5.60 USD` → `560`

### Balance parsing

- Same parsing rules as amount
- Used as a **reconciliation field** only — to detect missed messages
- Balance is forwarded in the payload but does not affect transaction classification

### Timestamp

- Parse as `YYYY-MM-DD HH:MM:SS` in `Asia/Tashkent` timezone
- Store and transmit as ISO 8601 UTC

### Merchant name

- Everything between `Pokupka: ` and the country code + datetime is the merchant field
- Preserve original casing and spacing; do not normalize
- Country code (3 uppercase letters) and datetime mark the end of the merchant field

### OTP / non-transaction messages

- Any SMS from `infinbank` that does not match the `Pokupka:` pattern must be **silently dropped**
- OTP messages must never be logged, stored, or transmitted — this is a security non-negotiable

---

## Idempotency

- Idempotency key: SHA-256 hash of the raw SMS message body + sender address
- The key is included in every outbound request per the finance app's API contract (field name defined in `specs/`)
- If the same message is received twice (e.g., delivery retry by the OS), the second POST will be rejected by the finance app as a duplicate — the app must treat a duplicate response as success and not retry

---

## Delivery and retry

- On successful POST (2xx): mark message as delivered, no further action
- On network failure or 5xx: enqueue for retry with **exponential backoff** (initial: 30s, max: 30 min)
- On 4xx (excluding duplicate): log as a parse or contract error, do not retry, alert via notification
- Retry queue is persisted in local Room database — survives app restart and device reboot
- Maximum retry age: 24 hours; after that, mark as permanently failed and notify user

---

## Reconciliation

- After each successful delivery, compare the parsed `balance` against the last known balance for the card
- If the gap between the new balance and (last balance − amount) exceeds a threshold (suggest: 1 UZS for UZS, 0.01 USD for foreign currency), log a warning — this indicates a missed message
- Reconciliation failures are surfaced as a non-blocking notification; they do not block delivery

---

## Background execution

- **Minimum SDK:** API 35 (Android 16)
- SMS is received via `BroadcastReceiver` on `SMS_RECEIVED` — real-time, no polling needed
- A **foreground service** with a persistent notification must wrap the retry queue processor to survive background restrictions
- On first launch, backfill inbox: read all existing SMS from `infinbank` sender and process any undelivered transactions
- Backfill is idempotent by design (same idempotency key scheme applies)

---

## Permissions required

| Permission | Reason |
|---|---|
| `RECEIVE_SMS` | Real-time SMS delivery via BroadcastReceiver |
| `READ_SMS` | Inbox backfill on first launch |
| `INTERNET` | POST to finance app ingest endpoint |
| `FOREGROUND_SERVICE` | Keep retry queue processor alive |
| `RECEIVE_BOOT_COMPLETED` | Re-register BroadcastReceiver after device reboot |

No notification listener permission in this version.

---

## Configuration (not hardcoded)

| Setting | Description |
|---|---|
| Finance app ingest endpoint URL | Base URL of the finance app API |
| Card-to-account mapping | Maps masked card number to finance app account ID |
| Reconciliation threshold per currency | Per-currency tolerance for balance gap detection |

Configuration is stored in encrypted `SharedPreferences` (Android Keystore-backed).

---

## Parse failure handling

- Any SMS from `infinbank` that passes the sender filter but fails to parse must be:
    1. Logged locally with raw message body (encrypted at rest)
    2. Surfaced as a notification: "1 bank message could not be parsed — tap to review"
    3. Never silently dropped
- The user can view raw failed messages in the app and manually dismiss them

---

## Out of scope for this version

- Push notification reading
- Multiple bank senders
- Multiple cards
- Manual transaction entry
- Any UI beyond: status indicator, failed parse review screen, settings screen

---

## Test cases (from real messages)

These must all parse correctly and produce the expected output:

| Input | Type | Amount (minor units) | Currency | Merchant |
|---|---|---|---|---|
| `Pokupka: YANDEX.GO YUNUSOBOD TUM UZB 2026-06-18 10:08:41, karta 404800***7476. summa: 28 500.00 UZS, balans: 19 315 740.00 UZS` | expense | 2850000 | UZS | `YANDEX.GO YUNUSOBOD TUM` |
| `Pokupka: ANTHROPIC +14152360599 USA 2026-06-17 07:39:15, karta 404800***7476. summa: 5.60 USD, balans: 19 344 240.00 UZS` | expense | 560 | USD | `ANTHROPIC +14152360599` |
| `Pokupka: HUMO MONEY TRANSFER Tashkent UZB 2026-06-17 10:33:25, karta 404800***7476. summa: 550 000.00 UZS, balans: 19 412 000.00 UZS` | expense | 55000000 | UZS | `HUMO MONEY TRANSFER Tashkent` |
