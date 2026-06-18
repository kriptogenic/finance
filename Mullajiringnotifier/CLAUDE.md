# CLAUDE.md — bank-sms-reader

## What this project is

An Android background service app (Kotlin, API 35+) that reads Infinbank SMS transaction notifications, parses them into structured data, and POSTs them to the personal finance manager app's ingest endpoint.

Read `REQUIREMENTS.md` fully before writing any code. Read the OpenAPI spec in `specs/` to understand the ingest endpoint contract before writing the HTTP layer.

---

## Non-negotiables

- **Never store, log, or transmit OTP messages.** Any SMS from `infinbank` that does not match the `Pokupka:` pattern is silently dropped. No exceptions, no fallback logging of raw content for non-matching messages.
- **Never use floating point for money.** All amounts are integer minor units (e.g. `28 500.00 UZS` → `2850000`). Use `Long` throughout. Never `Double` or `Float` for any monetary value.
- **Idempotency key on every request.** SHA-256 of (raw SMS body + sender address). Always included in the POST per the API contract in `specs/`.
- **Sender filter is the first gate.** Before any parsing, check sender == `infinbank` (case-insensitive). If it doesn't match, drop immediately — no further processing.
- **Encrypted storage.** Card-to-account mapping, endpoint URL, and any logged raw parse failures must be stored in encrypted SharedPreferences (Android Keystore-backed). Never plaintext.

---

## Stack

- **Language:** Kotlin only — no Java
- **Minimum SDK:** 35 (Android 16)
- **DI:** Hilt
- **Database:** Room (retry queue + parse failure log)
- **HTTP:** Ktor client (or Retrofit — pick one, don't mix)
- **Background:** WorkManager for retry queue, BroadcastReceiver for real-time SMS
- **Coroutines:** Kotlin coroutines + Flow throughout — no RxJava
- **Testing:** JUnit 5 + MockK

---

## Architecture

Follow clean architecture with these layers:

```
BroadcastReceiver (SMS_RECEIVED)
    └── SmsProcessor (domain)
            ├── SenderFilter
            ├── SmsParser
            ├── IdempotencyKeyGenerator
            └── TransactionRepository
                    ├── RetryQueue (Room)
                    └── FinanceApiClient (Ktor/Retrofit)
```

No business logic in BroadcastReceiver or ViewModel. BroadcastReceiver only receives and hands off to SmsProcessor.

---

## Build order

Do not skip ahead. Complete and test each step before moving to the next:

1. **SmsParser** — pure functions, no Android dependencies, fully unit tested against the test cases in `REQUIREMENTS.md`
2. **SenderFilter** — trivial, but write the test
3. **IdempotencyKeyGenerator** — SHA-256, unit tested
4. **Room schema** — RetryQueue table + ParseFailureLog table
5. **FinanceApiClient** — HTTP POST only, reads contract from `specs/`
6. **SmsProcessor** — wires the above together
7. **BroadcastReceiver** — wires SmsProcessor, registers for SMS_RECEIVED
8. **ForegroundService** — wraps retry queue processor
9. **Backfill** — on first launch, read inbox and process existing messages
10. **UI** — status screen, failed parse review screen, settings screen (last)

---

## Parser rules (critical)

- Input format: `Pokupka: {merchant} {COUNTRY_3} {YYYY-MM-DD HH:MM:SS}, karta {masked_card}. summa: {amount} {currency}, balans: {balance} {currency}`
- Amount/balance: strip space thousands separators, replace comma decimal with period, parse as BigDecimal, multiply by 100, convert to Long
- Timestamp: parse as `Asia/Tashkent`, store/transmit as ISO 8601 UTC
- Merchant: everything between `Pokupka: ` and the 3-letter uppercase country code — preserve original casing and spacing
- Country code boundary: first occurrence of a standalone 3-letter uppercase word followed by a datetime pattern
- All 4 real examples in `REQUIREMENTS.md` must pass as unit tests before the parser is considered done

---

## Error handling

- Parse failure: log to Room ParseFailureLog (encrypted raw body), post a user notification, do not crash
- Network failure / 5xx: enqueue in Room RetryQueue with exponential backoff (30s initial, 30min max, 24h TTL)
- 4xx (non-duplicate): log as contract error, do not retry, notify user
- Duplicate response from finance app: treat as success, do not retry
- Reconciliation mismatch (balance gap): log warning + notify user, do not block delivery

---

## What not to do

- No push notification listener in this version — do not scaffold it
- No multi-bank support — sender filter is hardcoded to `infinbank` for now
- No multiple card support — one card in config for now
- No manual transaction entry UI
- No analytics, no crash reporting SDKs, no third-party logging
- Do not use `Thread.sleep` anywhere — use coroutines
- Do not define the API contract — it is in `specs/`; read it, don't rewrite it