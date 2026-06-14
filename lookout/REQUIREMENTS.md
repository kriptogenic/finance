# Telegram Bank-Notification Userbot — Requirements

> Spec for a **Go** service that reads bank-notification messages from a single
> Telegram DM and feeds them into the **Personal Finance Manager** app (see that
> app's `REQUIREMENTS.md`) as transactions. Written to be handed to Claude Code.
> Build incrementally per the phased scope in §11.

---

## 1. Overview

A single-user Telegram **userbot** (MTProto user session — **not** the Bot API)
written in Go. It watches one chat: the DM between the user's own Telegram account
and a **bank notification bot** (Humo card alerts). For each transaction
notification it:

1. reads the message,
2. parses it into a structured record,
3. maps it onto the finance app's transaction model, and
4. posts it to the finance app's ingest API.

Latency is **not critical** — handling a message a minute or more after it arrives
is acceptable.

**Design priorities, in order:** never double-post a transaction · never silently
drop a message · correct money math and transaction-type mapping · respect the
finance app's invariants (especially *transfers don't change net worth*).

---

## 2. Why a userbot (not a Bot API bot)

The message source is a **DM with another bot**. The Bot API cannot read arbitrary
private chats, and bots are not allowed to talk to other bots — only the **user
account** can see these messages. A user session over MTProto is therefore required.

- **Library:** `gotd` (`github.com/gotd/td`) — pure Go, no cgo, single static binary.
- **Read-only on Telegram.** The bot never sends Telegram messages. This is the
  lowest-risk userbot behavior. ToS caveat: this is a real user account; keep it
  passive (polling reads only) to minimize any limit/ban risk.

---

## 3. Telegram ingestion

- **Auth:** one-time interactive login (phone number, login code, and 2FA password
  if set). Persist the session to a session file; reconnect silently afterward.
  `API_ID` / `API_HASH` come from my.telegram.org via config.
- **Peer resolution:** resolve the source bot's peer **once** (by username/ID from
  config) and cache its access hash.
- **Polling, not live updates.** Poll message history on the peer every
  `POLL_INTERVAL` (default 30–60s). Process messages with ID greater than the last
  processed watermark; advance and persist the watermark. On restart, resume from
  the stored watermark — history fetch naturally backfills anything missed.
  - *Rationale:* because latency is relaxed, polling lets us avoid MTProto
    update-gap recovery (`pts/qts/seq`, `getDifference`) entirely, which is simpler
    and more robust for a "must not miss messages" feed.
- **Edits — OPEN DECISION (see §13.1).** Some bank bots send a placeholder and then
  *edit* it into the final value. If the source bot does this, an append-only
  by-ID poll will miss the edit; the poller must instead re-scan a small recent
  window each cycle and detect changes via `edit_date`/content. Default assumption:
  notifications are fresh messages, not edited.
- **Flood-wait:** trivial at this poll rate, but still wrap Telegram calls with
  gotd's floodwait/ratelimit middleware.

---

## 4. Message parsing

### 4.1 Format

Messages are **emoji-delimited with a fixed field order**:

```
[💸/🎉 type-word] [➖/➕ amount UZS] [📍 merchant] [💳 card] [🕓 time date] [💰 balance UZS]
```

Real samples (use as test fixtures). **Field separators vary** — some messages are
single-line (fields run together), others put each field on its own line. The parser
must be whitespace/newline-tolerant: anchor on the emoji markers and allow any
whitespace (incl. `\n`) around them.

Single-line:

```
💸 Оплата➖ 57.550,00 UZS📍 SP OOO HAVAS FOOD>T💳 HUMOCARD *4853🕓 10:03 14.06.2026💰 697.945,26 UZS
🎉 Пополнение➕ 520.000,00 UZS📍 DAVR MOBILE P2P U2H>💳 HUMOCARD *4853🕓 23:17 13.06.2026💰 2.088.245,26 UZS
💸 Операция➖ 1.000.000,00 UZS📍 TBC HUMO P2P>TASHKEN💳 HUMOCARD *4853🕓 09:36 14.06.2026💰 1.088.245,26 UZS
🎉 Пополнение➕ 1.000.000,00 UZS📍 TBC HUMO P2P>TASHKEN💳 HUMOCARD *8400🕓 09:36 14.06.2026💰 1.110.241,56 UZS
```

Multi-line (newline-delimited) — a **transfer to another person** → a lone debit, so
it maps to an **expense**, not a transfer (no matching credit on my cards, §5.1):

```
💸 Оплата
➖ 500.000,00 UZS
📍 TBC P2P S HUMO NA UZ
💳 HUMOCARD *8400
🕓 09:39 14.06.2026
💰 610.241,56 UZS
```

### 4.2 Parsing rules

- **Anchor on the emoji markers** (`➖/➕`, `📍`, `💳`, `🕓`, `💰`), not on the field
  contents. Field order is stable; one tolerant regex captures all fields. Allow
  **arbitrary whitespace including newlines** around every marker (messages come both
  single-line and one-field-per-line) — use `(?s)` and `\s*` between fields.
- **Emoji robustness:** allow an optional variation selector (`U+FE0F`) after each
  marker emoji — senders may or may not include it, and an exact-byte match will
  silently break otherwise.
- **Direction** comes from `➖` (debit) / `➕` (credit) — **never** from the word.
  The type word set `{Оплата, Операция, Пополнение}` is descriptive, not a stable
  enum, and `Операция` is reused for the outgoing leg of a transfer.
- **Amount & balance** use European/Uzbek formatting: `.` = thousands separator,
  `,` = decimal separator. Parse to **integer minor units** (×100) as `int64`.
  **Never use float for money** — this mirrors the finance app's money rule.
  Example: `697.945,26` → `69794526`.
- **Card:** `"<TYPE> *<last4>"` → extract `card_type` (e.g. `HUMOCARD`) and
  `card_last4` (e.g. `4853`).
- **Datetime:** layout `15:04 02.01.2006`, **minute precision** (no seconds).
  Interpret in **Asia/Tashkent** (set explicitly; default `TIMEZONE=Asia/Tashkent`)
  and emit as a TZ-aware timestamp.
- **Merchant:** truncated and lossy — the bank cuts it at a fixed length, and `>`
  separates merchant from city/terminal but is often chopped. **Store raw; never use
  as a key.** Use only as a hint for optional category rules (prefix/fuzzy).
- **Balance after:** capture as `int64` minor units — used for reconciliation (§8).
- **Fail loud:** if a message cannot be fully parsed, **log it and still forward it**
  with the raw text and an `parsed=false` flag — never drop it. Bank formats drift;
  degrade to raw passthrough, not silent data loss.
- Always retain the **original message text** and the **Telegram message ID** in the
  record.

---

## 5. Mapping to the finance app's bucket model

The finance app models every transaction as money moving between two buckets, with
`type ∈ {expense, income, transfer}`, integer-minor-unit amounts, a currency, a
frozen `rate_to_base`, and `from`/`to` buckets. The bot maps each parsed message
onto that model:

| Parsed message | App transaction `type` | `from` bucket | `to` bucket | Category |
|---|---|---|---|---|
| Debit (`➖`, Оплата/Операция) | **expense** | the card's **account** | expense **category** | required (§6) |
| Credit (`➕`, Пополнение) | **income** | income **category** (source) | the card's **account** | required (§6) |
| Internal transfer (paired debit+credit on two of the user's own cards) | **transfer** | source card account | destination card account | **none** |

### 5.1 Transfers are the critical case

**Every message is for one of *my* cards only** — the notification shows my
`card_last4`, never the counterparty's. So a lone debit is ambiguous on its own: the
money could have gone to my *other* card or to another person. The **only** signal
that it was an internal transfer is that a **matching credit appears on another of my
cards**. That existence *is* the discriminator:

| What I receive | Meaning | Post as |
|---|---|---|
| debit on card A **and** credit on card B (A≠B, equal amount, 🕓 within window) | transfer between my own cards | **one `transfer`** |
| debit only, no matching credit (e.g. `➖ … 📍 TBC P2P S HUMO NA UZ 💳 *8400`) | sent to another person | **expense** |
| credit only, no matching debit | received from another person | **income** |

Therefore `CARD_ACCOUNT_MAP`'s job is **not** to gate "is this mine" (every message
is mine) — it is to (a) resolve each card's `account_id` and (b) enumerate all my
cards. An internal transfer is recognized purely by **both legs appearing**.

If the two legs were posted independently as expense + income, the app would
double-count and **break net worth** — violating the invariant that *transfers carry
no category and don't change net worth*. They **must collapse into ONE `transfer`**
(from = debit's account, to = credit's account, shared `transfer_group_id`,
deterministic `external_id = tg:transfer:<id_lo>-<id_hi>`).

**Fees are always a separate message/transaction.** So the two transfer legs are
clean and **exactly equal** → match on exact amount (no fuzzy tolerance). The fee
arrives as its own small debit (different amount → never pairs) → standalone expense.

**Pairing rule.** Hold every debit/credit briefly and look in the buffer for a
counterpart: **opposite sign, equal amount, different own-card, |🕓Δ| ≤
`TRANSFER_PAIR_WINDOW`**. On a match → emit one `transfer` and drop both legs.
Two distinct timers:

- `TRANSFER_PAIR_WINDOW` (≈60–120s) — max gap between the two **🕓 transaction
  times**. Legs are usually same-minute but can arrive up to ~2 min apart.
- `HOLD_DURATION` (≈5 min, **> poll interval + skew**) — how long an unmatched leg
  waits before it's flushed as a standalone expense/income. Must exceed poll latency
  so a leg never times out *before* its mate is even polled (this is the safeguard
  against double-counting).

The buffer must tolerate **out-of-order arrival** (match against the buffer, so order
is irrelevant), persist pending legs across restarts (§8), and guarantee
**at-most-once** posting (idempotent `external_id`).

**False-positive risk.** A *coincidental* equal-amount expense + income within the
window would look like a transfer and be wrongly merged (net worth off). Guards, in
order: keep the 🕓 window tight; use the merchant marker as a **confirmation** —
internal own-transfers seem to carry a self-transfer hint (`U2H` in
`DAVR MOBILE P2P U2H`, vs the external `… P2P S HUMO NA UZ`), tune against real data;
optionally cross-check `balance_after` (card A drops by X, card B rises by X). When
unsure, lean **conservative** — post expense + income and let the user re-merge in the
app — and **log every auto-merge** for review.

### 5.2 Currency

All messages are **UZS**. Set `currency=UZS`. Per the app's frozen-rate rule, supply
`rate_to_base` at post time: if `BASE_CURRENCY=UZS` then `rate_to_base=1`; otherwise
the bot must obtain/compute a rate (config or an FX source) — see §13.4.

---

## 6. Responsibility split (bot vs app) — RESOLVED

The bot is a **thin adapter**: parse messages, pair transfer legs (the one job that
needs the temporal stream), and forward. The **app is the system of record** and owns
all domain identity and rules.

| Concern | Owner | How |
|---|---|---|
| Parse Telegram messages | **bot** | §4 |
| **Pair transfer legs** → one transfer | **bot** | §5.1 — needs the time-bounded buffer; keeps phantom expense+income out of the ledger |
| **Card → account** | **app** | each account stores its `card_last4` (set in the app UI); ingest resolves it. Unknown card → `400`. |
| **Merchant → category** | **app** | ingest matches `merchant` against the app's `category_rules`, else the built-in `Uncategorized` bucket (`system_key`); transfers get none |
| **Dedup** | **app** | idempotent on `external_id` |

So the bot needs **no** card→account map and **no** category ids — it sends the card
last4(s) and the raw merchant string; the app routes. The bot still pairs on
`card_last4` (every message is one of my cards, §5.1).

---

## 7. Integration contract (bot → finance app)

The finance app exposes (built):

```
POST {FINANCE_API_URL}/ingest/transactions
Authorization: Bearer {FINANCE_API_TOKEN}
Content-Type: application/json
```

Payload (validated against the app's OpenAPI `IngestTransactionRequest`):

```jsonc
{
  "external_id": "tg:<chat_id>:<message_id>",   // REQUIRED — idempotency key
                                                // transfers: "tg:transfer:<id_lo>-<id_hi>"
  "type": "expense" | "income" | "transfer",
  "from_card_last4": "4853",                    // expense + transfer (money leaves this card)
  "to_card_last4": "8400",                      // income + transfer (money enters this card)
  "merchant": "SP OOO HAVAS FOOD>T",            // raw; app routes to a category, stores as note
  "amount": 5755000,                            // int64 minor units, primary leg
  "to_amount": 125000000,                       // transfers only, cross-currency (omit for UZS)
  "rate_to_base": "1",                          // string decimal; omit when currency == base
  "date": "2026-06-14T10:03:00+05:00",          // Asia/Tashkent; defaults to now
  "tags": ["humo"],
  "transfer_group_id": "…"                       // optional, set on both paired legs
}
```

- **Card → account, merchant → category** are resolved **server-side** (§6). The bot
  sends `from_card_last4`/`to_card_last4` and `merchant`, never account/category ids.
- **Currency is derived from the resolved account.** With `BASE_CURRENCY=UZS` the app
  accepts the post with no `rate_to_base`; send it only for non-base legs.
- **Idempotency:** dedupe on `external_id` (partial-unique). **`201`** = created,
  **`200`** = already existed (returns the stored transaction unchanged). Both are
  success → advance the watermark.
- **Auth:** bearer `INGEST_TOKEN`. Empty token on the app side disables auth (local);
  otherwise a mismatch is `401`.
- **Delivery:** POST, generous exponential backoff. Advance the watermark **only after
  a 201/200** so a crash mid-delivery re-sends.
- **Not sent to the app:** `balance_after`, `raw_text`, `parsed` — bot-side only. A
  message that fails to parse (`parsed=false`) is **not** posted; the bot logs/queues
  it for the operator (the app feed is insert-only and strongly-typed).

---

## 8. Reliability & ops

- **Persisted state:** last-processed message-ID watermark; **pending unpaired
  transfer legs** (persist these so a restart doesn't lose half a transfer);
  delivery progress (or rely on the watermark + app idempotency).
- **Reconciliation / gap detection:** for each card, `balance_after` of message *N*
  should equal that of *N−1* ± its amount. On mismatch, log a gap and optionally
  backfill via a history fetch. This detects dropped/missed messages even with
  polling.
- **Logging:** structured; explicit errors on parse failure and delivery failure.
- **Graceful shutdown:** finish in-flight delivery, persist state, flush the pairing
  buffer's pending legs.
- **Packaging:** single static binary; config via env/flags; no secrets in code.
- **Security/PII:** the session file and API token are secrets — document storage
  expectations. Log only `card_last4`, never full card data; avoid logging full
  message PII beyond what's needed to debug.

---

## 9. Configuration (env)

| Var | Purpose |
|---|---|
| `TELEGRAM_API_ID`, `TELEGRAM_API_HASH` | from my.telegram.org |
| `SESSION_FILE` | persisted user session path |
| `SOURCE_BOT` | source bot username or ID |
| `POLL_INTERVAL` | poll cadence (default 60s) |
| `TRANSFER_PAIR_WINDOW` | max gap between the two legs' 🕓 times to pair (≈120s) |
| `TRANSFER_HOLD_DURATION` | how long to hold an unmatched leg before flushing it standalone (≈5m) |
| `FINANCE_API_URL`, `FINANCE_API_TOKEN` | ingest endpoint + bearer token (= app's `INGEST_TOKEN`) |
| `TIMEZONE` | default `Asia/Tashkent` |

Card→account and merchant→category are **app-side** (§6), so the bot no longer needs
`CARD_ACCOUNT_MAP`, `DEFAULT_*_CATEGORY_ID`, or `BASE_CURRENCY`. Set each account's
`card_last4` in the finance app instead.

---

## 10. Out of scope / non-goals

- Multiple chats or multiple source bots — **one chat only**.
- Sending Telegram messages — **read-only**.
- Real-time / low-latency delivery.
- Smart categorization beyond simple optional rules (defer to the app).
- Editing or deleting transactions in the app — this is an **insert-only feed**;
  corrections happen in the app.

---

## 11. Phasing / build order

1. **Loop:** Telegram session + polling + persisted watermark; parse one message
   type (expense); post to a stub endpoint. Prove end-to-end.
2. **Parser:** all directions, integer-minor-unit money, datetime/TZ, emoji
   tolerance, fail-loud + raw passthrough. Test against the §4.1 fixtures.
3. **Integration:** ingest contract + idempotency + retries; `CARD_ACCOUNT_MAP`;
   expense/income posting.
4. **Transfers:** pairing buffer → single `transfer`; persist pending legs;
   out-of-order + timeout handling.
5. **Hardening:** balance reconciliation / gap detection; ops, logging, graceful
   shutdown.

---

## 12. Notes for Claude Code

- **Money is `int64` minor units everywhere; never float** — mirror the finance
  app's invariant.
- Treat **all message content as untrusted**; the parser must never panic on an
  unexpected format — it falls back to `parsed=false` raw passthrough.
- **Tests are mandatory** for: the parser against the §4.1 samples (number format,
  truncated merchant, both directions); transfer pairing (in-order, out-of-order,
  no-mate timeout); idempotency (same message twice → one transaction); balance
  reconciliation gap detection.
- Keep Telegram ingestion, parsing, mapping, and delivery as **separate packages**
  so a slow/flaky ingest endpoint can't stall the poll loop.

---

## 13. Open decisions to confirm before building

1. **Does the source bot edit messages** (placeholder → final value)? Determines the
   polling strategy in §3. *(Still open — observe the real feed.)*
2. **Category strategy** (§6): ✅ **resolved — app-side.** The app routes `merchant` →
   a `category_rules` match, else the built-in `Uncategorized` bucket. The bot sends
   the raw merchant; no bot-side category config.
3. **Does the finance app expose an ingest endpoint** (§7)? ✅ **resolved — built.**
   `POST /ingest/transactions`, bearer `INGEST_TOKEN`, idempotent on `external_id`
   (201 created / 200 deduped); resolves `card_last4`→account and `merchant`→category.
4. **Is UZS the app's base currency** (§5.2)? ✅ **resolved — yes** (`BASE_CURRENCY=UZS`,
   so `rate_to_base` is omitted/1).
5. **`CARD_ACCOUNT_MAP` values** — the actual last4 → account_id pairs. *(Operator
   config; gather the real card last4s and create/point at finance-app accounts.)*

---

## 14. Developing the bot — local setup, contract, conventions

Everything below is the context needed to start coding the bot in `lookout/`.

### 14.1 Project layout (monorepo)

```
finance/
  core/      Go finance app + API (DDD template; pgx, oapi-codegen, fx, zap, golang-migrate)
  frontend/  Vue 3 + Vite + TS SPA (openapi-typescript client)
  lookout/   ← THIS bot (Go userbot). Its own Go module.
  specs/     OpenAPI: api.yaml + definitions.yaml (source of truth for BOTH ends)
```

The app and frontend both generate their clients/types from `specs/`. **The bot
should do the same** — generate its ingest HTTP client from `specs/api.yaml`
(operation `ingestTransaction`, schema `IngestTransactionRequest`) with
`oapi-codegen`, so the contract can never drift.

### 14.2 Run the finance app locally (the ingest target)

```bash
# from repo root: Postgres (mapped to host :5433)
docker compose up -d

# from core/: schema + demo data, then run the API on :8080 with auth
cd core
make migrate-fresh && make seed
INGEST_TOKEN=secret123 make run
```

Seed gives the bot something to hit: debit accounts **Humo *4853** and **Humo *8400**
(UZS), and the **Uncategorized** (expense) + **Uncategorized income** buckets. The
`category_rules` table is empty by default (everything falls back to Uncategorized).

Smoke-test the endpoint:

```bash
curl -s -X POST http://localhost:8080/ingest/transactions \
  -H 'Authorization: Bearer secret123' -H 'content-type: application/json' \
  -d '{"external_id":"tg:1:100","type":"expense","from_card_last4":"4853",
       "amount":5755000,"merchant":"SP OOO HAVAS FOOD>T"}'
# 201 created; repeat -> 200 deduped; unknown card -> 400; bad/no token -> 401
```

Response status semantics (all the bot needs):
- **201** created · **200** already ingested (deduped) → both success, advance watermark.
- **400** unknown card / invalid shape · **401** missing/wrong bearer token.

### 14.3 Bot package layout & libraries

Separate Go module (`go mod init <module>/lookout`, Go 1.26). Keep the four stages
in **separate packages** so a slow ingest can't stall polling (§12):

| Package | Role |
|---|---|
| `config` | env via `ilyakaznacheev/cleanenv` (§9 vars) |
| `telegram` | `gotd/td` user session, peer resolve, history polling, watermark |
| `parser` | message text → structured record (§4); pure, fixture-tested |
| `pairing` | transfer leg buffer (two timers, out-of-order, persist pending) (§5.1) |
| `delivery` | ingest client generated from `specs/api.yaml`; retries; 201/200 = ok |
| `store` | persisted watermark + pending legs (file or embedded KV) |

Conventions mirrored from `core`: structured logging with **zap**; single static
binary (`CGO_ENABLED=0`); config via env/flags, **no secrets in code**; `gofmt` +
`golangci-lint`.

### 14.4 Tests are mandatory (§12)

- **parser** against the §4.1 fixtures — both single-line and newline formats, both
  directions, `697.945,26 → 69794526`, truncated merchant, Asia/Tashkent, and a
  fail-loud `parsed=false` case.
- **pairing** — in-order, out-of-order, no-mate timeout, and the false-positive guard.
- **idempotency** — same message twice → one transaction (rely on app dedupe via a
  stable `external_id`; transfer key = `tg:transfer:<id_lo>-<id_hi>`).
- **reconciliation** — `balance_after` gap detection.

### 14.5 Build order (recap of §11)

Parser first (pure, highest value) → pairing buffer → delivery/idempotency →
Telegram session + polling + watermark wired last → hardening (reconciliation, ops).
