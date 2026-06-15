# lookout

A single-user **Telegram bank-notification userbot** (Go) that reads Humo card
alerts from one Telegram DM, parses them, pairs internal transfers, and posts
transactions to the [finance app](../core)'s ingest API. Read-only on Telegram,
idempotent on the app side. Full spec: [`REQUIREMENTS.md`](./REQUIREMENTS.md).

## How it works

```
Telegram DM ──poll──▶ parser ──▶ pairing buffer ──▶ delivery ──▶ POST /ingest/transactions
   (gotd)            (§4)        (collapse transfers, §5.1)    (generated client, §7)
```

Each stage is a separate package so a slow ingest can't stall polling (§12):

| Package | Role |
|---|---|
| `internal/config`   | env config (cleanenv) |
| `internal/telegram` | gotd user session, peer resolve, history polling |
| `internal/parser`   | message text → structured record; pure, fixture-tested |
| `internal/pairing`  | transfer-leg buffer (two timers, out-of-order, persistable) |
| `internal/delivery` | ingest client generated from `../specs/api.yaml`; retries |
| `internal/store`    | atomic JSON state: watermark + pending legs |
| `internal/recon`    | balance-gap detection |
| `internal/app`      | orchestrator: poll loop, persist-after-deliver |

**Design priorities:** never double-post · never silently drop · correct money
math (int64 minor units, never float) · respect the app's invariant that
*transfers don't change net worth*.

## Run

1. `cp .env.example .env` and fill it in (Telegram `API_ID`/`API_HASH` from
   [my.telegram.org](https://my.telegram.org), the source bot username, and the
   finance app URL + `INGEST_TOKEN`).
2. Start the finance app (the ingest target) — see [`../core`](../core) /
   `REQUIREMENTS.md` §14.2.
3. First run performs a one-time interactive login (phone, code, 2FA) and writes
   the session file; afterwards it reconnects silently.

```bash
make run            # generate client + run
make build          # static binary → dist/lookout
make test           # all tests
make generate       # regenerate ingest client from ../specs
```

## Notes

- **Decoupling:** the bot integrates with the app only via `../specs/api.yaml`
  (generated client) + HTTP. It shares no business entities; it does reuse the
  shared `finance/pkg/money` value object via a monorepo `replace`.
- **State files** (`session.json`, `lookout-state.json`) and `.env` are secrets /
  local state and are gitignored.
