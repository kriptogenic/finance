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

Real samples (use as test fixtures):

```
💸 Оплата➖ 57.550,00 UZS📍 SP OOO HAVAS FOOD>T💳 HUMOCARD *4853🕓 10:03 14.06.2026💰 697.945,26 UZS
🎉 Пополнение➕ 520.000,00 UZS📍 DAVR MOBILE P2P U2H>💳 HUMOCARD *4853🕓 23:17 13.06.2026💰 2.088.245,26 UZS
💸 Операция➖ 1.000.000,00 UZS📍 TBC HUMO P2P>TASHKEN💳 HUMOCARD *4853🕓 09:36 14.06.2026💰 1.088.245,26 UZS
🎉 Пополнение➕ 1.000.000,00 UZS📍 TBC HUMO P2P>TASHKEN💳 HUMOCARD *8400🕓 09:36 14.06.2026💰 1.110.241,56 UZS
```

### 4.2 Parsing rules

- **Anchor on the emoji markers** (`➖/➕`, `📍`, `💳`, `🕓`, `💰`), not on the field
  contents. Field order is stable; one tolerant regex captures all fields.
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

A transfer between the user's own cards arrives as **two separate messages** — a
debit on the source card and a credit on the destination card, with the **same
amount, same minute, opposite signs, different cards** (see the §4.1 samples).

These two messages **must be collapsed into ONE `transfer`** in the finance app. If
posted independently as an expense + an income, the app would record a phantom
expense and a phantom income, **double-count, and break net worth** — violating the
app invariant that *transfers carry no category and don't change net worth*.

**Pairing rule:** hold any debit/credit whose card is one of the user's own
accounts for up to `TRANSFER_PAIR_WINDOW` seconds, looking for a counterpart message
with equal amount, opposite sign, same minute, and a second own-card. On a match →
emit a single `transfer` (assign both legs a shared `transfer_group_id`). If no
counterpart arrives within the window → emit the standalone expense/income. The
buffer must tolerate **out-of-order arrival** and guarantee **at-most-once** posting.

### 5.2 Currency

All messages are **UZS**. Set `currency=UZS`. Per the app's frozen-rate rule, supply
`rate_to_base` at post time: if `BASE_CURRENCY=UZS` then `rate_to_base=1`; otherwise
the bot must obtain/compute a rate (config or an FX source) — see §13.4.

---

## 6. Category assignment — OPEN DECISION (§13.2)

The app requires expenses/income to have a category, but truncated merchant strings
can't be reliably categorized. Options, simplest first:

- **(a) Recommended for MVP:** post into a dedicated **"Uncategorized / Needs
  review"** category; the user re-categorizes inside the app. Requires the app to
  have such a category (or the ingest endpoint to accept a *pending/uncategorized*
  state). *App-side prerequisite.*
- **(b)** Bot applies a config-driven rule table (merchant prefix → category),
  falling back to Uncategorized.
- **(c)** Bot posts `merchant_raw` only and the **app** assigns the category via its
  own (planned) auto-categorization feature.

---

## 7. Integration contract (bot → finance app)

The finance app **now exposes** this endpoint (built; see that app's commit
"ingest endpoint"):

```
POST {FINANCE_API_URL}/ingest/transactions
Authorization: Bearer {FINANCE_API_TOKEN}
Content-Type: application/json
```

The bot resolves cards → account ids (via `CARD_ACCOUNT_MAP`) and chooses the
category id (the Uncategorized buckets, §6) **before** posting — the app does not
know about cards. Payload (the app validates against its OpenAPI `IngestTransactionRequest`):

```jsonc
{
  "external_id": "tg:<chat_id>:<message_id>",   // REQUIRED — idempotency key
  "type": "expense" | "income" | "transfer",
  "from_account_id": "<uuid>",                  // per type rules (§5)
  "to_account_id": "<uuid>",
  "category_id": "<uuid>",                       // required for expense/income; omit for transfer
  "amount": 5755000,                             // int64 minor units, primary leg
  "to_amount": 125000000,                        // transfers only, cross-currency (omit for UZS)
  "rate_to_base": "1",                           // string decimal; omit when currency == base
  "date": "2026-06-14T10:03:00+05:00",           // Asia/Tashkent; defaults to now
  "note": "SP OOO HAVAS FOOD>T",                 // put merchant_raw here
  "tags": ["humo", "*4853"],
  "transfer_group_id": "…"                        // optional, set on both paired legs
}
```

- **Currency is derived from the account** server-side (all UZS here), so the bot
  doesn't send `currency`. With `BASE_CURRENCY=UZS` the app accepts the post with
  no `rate_to_base` (treated as base); send `"rate_to_base"` only for non-base legs.
- **Idempotency:** the app dedupes on `external_id` (partial-unique). Response is
  **`201`** when created, **`200`** when it already existed (returns the stored
  transaction unchanged). Either is a success → advance the watermark.
- **Auth:** bearer token = `INGEST_TOKEN` on the app side. If the app's token is
  empty (local-only), auth is skipped; otherwise a mismatch is `401`.
- **Delivery:** HTTP POST, retry with generous exponential backoff. Advance the
  message watermark **only after a 201/200** so a crash mid-delivery re-sends.
- **Not sent to the app:** `balance_after`, `raw_text`, `parsed`, `card_*` — these
  stay bot-side (reconciliation/logging). A message that fails to parse
  (`parsed=false`) is **not** posted to the transaction endpoint; the bot logs/queues
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
| `TRANSFER_PAIR_WINDOW` | seconds to hold a leg waiting for its mate |
| `FINANCE_API_URL`, `FINANCE_API_TOKEN` | ingest endpoint + auth |
| `BASE_CURRENCY` | e.g. `UZS` (drives `rate_to_base`) |
| `TIMEZONE` | default `Asia/Tashkent` |
| `CARD_ACCOUNT_MAP` | `last4 → finance-app account_id` (e.g. `4853:acc_x,8400:acc_y`). **Required** both to set `from`/`to` accounts and to know which cards are "mine" for transfer detection. |
| `DEFAULT_EXPENSE_CATEGORY_ID`, `DEFAULT_INCOME_CATEGORY_ID` | the "Uncategorized" buckets (§6a) |

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
2. **Category strategy** (§6): ✅ **resolved — option (a).** The finance app seeds an
   `Uncategorized` (expense) and `Uncategorized income` (income) category; point
   `DEFAULT_EXPENSE_CATEGORY_ID` / `DEFAULT_INCOME_CATEGORY_ID` at their ids.
3. **Does the finance app expose an ingest endpoint** (§7)? ✅ **resolved — built.**
   `POST /ingest/transactions`, bearer `INGEST_TOKEN`, idempotent on `external_id`
   (201 created / 200 deduped).
4. **Is UZS the app's base currency** (§5.2)? ✅ **resolved — yes** (`BASE_CURRENCY=UZS`,
   so `rate_to_base` is omitted/1).
5. **`CARD_ACCOUNT_MAP` values** — the actual last4 → account_id pairs. *(Operator
   config; gather the real card last4s and create/point at finance-app accounts.)*
