# Finance

A private, single-user, multi-currency personal finance app: accounts, loans,
transfers, and income/expense tracking by category, with net-worth, spending,
and cash-flow reporting. Bank notifications are ingested automatically from
Telegram.

Money is always stored as **integer minor units + ISO-4217 currency code**
(never floating point), and every non-base-currency transaction freezes its
`rate_to_base` at the time it is recorded so reports never re-convert at a live
rate.

## Architecture

A monorepo of three independently deployable services that integrate **only**
through the OpenAPI contract in `specs/` — they never share Go types.

| Service     | Stack                                  | Role                                                            |
|-------------|----------------------------------------|-----------------------------------------------------------------|
| `core/`     | Go · uber-fx · chi · pgx · go-money    | Backend, REST API, money core. System of record.               |
| `frontend/` | Vue 3 · Vite · Tailwind · openapi-fetch | PWA single-page app.                                            |
| `lookout/`  | Go · gotd (Telegram userbot)           | Reads bank notifications, parses them, posts to the ingest API. |

`specs/api.yaml` (paths + schemas) and `specs/definitions.yaml` (shared enums)
are the single source of truth; they drive Go codegen for `core`/`lookout` and
TypeScript codegen for `frontend`.

## Implemented features

### Accounts
- Five account types across two kinds: **assets** (`cash`, `debit_card`,
  `deposit`) and **liabilities** (`credit_card`, `loan`).
- Each account has its own currency and an opening balance; the current balance
  is **derived** from transactions, never stored directly.
- Type-specific metadata: credit limit (credit cards); interest rate, term,
  principal, start/maturity dates, payment day, and capitalization (loans &
  deposits).
- A `card_last4` identifier routes external bank-notification ingest to the
  right account.
- Accounts with transactions are **archived, never hard-deleted**; deletion is
  rejected (409) when an account is in use.

### Categories
- Separate **expense** and **income** category trees, with subcategories
  (`parent_id`), icons (Tabler webfont), and colors.
- Optional **LLM-assisted icon suggestions** for a category name (Anthropic;
  silently disabled when no API key is configured).
- Archived rather than deleted once referenced by transactions.

### Transactions
- Three types: **expense**, **income**, and **transfer**.
  - Transfers carry no category and **don't change net worth**.
  - Expenses/income require a category whose `type` matches.
- **Cross-currency transfers**: separate `to_amount` in the destination
  currency; non-base amounts freeze a `rate_to_base`.
- Full edit (`PUT`, re-validates and re-freezes the rate) and quick
  re-categorize (`PATCH`).
- Search and filter by account, category, type, date range, tag, free-text
  note, or the **uncategorized** bucket; paginated.
- Tags and notes.

### Ingest (bank notifications)
- **Idempotent** `POST /ingest/transactions` keyed by a stable `external_id` —
  re-posting the same notification is deduped, never double-counted.
- The app owns routing: callers send `card_last4` + raw merchant text; core
  resolves cards → accounts and the merchant → a category.
- **Merchant → category routing rules** (case-insensitive substring match),
  fully CRUD-managed.
- **Category suggestions** for a transaction: local rules first, then an
  optional LLM fallback.
- **Rule blocks**: mark a merchant so the app never offers to create a routing
  rule for it again.
- Separate **bearer-token** auth for the ingest endpoints (distinct from the UI's
  Basic auth).

### Reconciliation
- `POST /ingest/balances` accepts periodic bank-reported card balance snapshots
  (upserted per card).
- `GET /reconciliation` compares each reported balance against the
  transaction-derived balance, flagging out-of-sync rows and currency
  mismatches.

### Loans
- Amortization schedule per loan account: monthly payment, total payment, total
  interest, and a full per-period principal/interest/balance breakdown.

### Budgets
- Per-expense-category budgets with **weekly / monthly / yearly** periods.
- Live progress for the current period: spent, remaining, percent, and period
  bounds; optional **rollover** of unused amounts and a custom start period.

### Scheduled (recurring) transactions
- A saved transaction template (expense, income, or transfer) plus a recurrence
  rule: **frequency** (daily / weekly / monthly / yearly) × **interval** (every
  N units), anchored to a next-run date with an optional end date.
- A background worker materializes due schedules into real transactions through
  the same ledger engine as manual entries, so they honor every money invariant
  (frozen `rate_to_base`, bucket shape, net-worth rules). Missed occurrences post
  once and skip ahead rather than backfilling a burst.
- Pause/resume, edit, delete, and a **Run now** action to materialize an
  occurrence immediately.

### Reports
- **Net worth**: Σ assets − Σ liabilities in the base currency, with
  per-currency exposure and a list of currencies missing a known rate (excluded
  from totals).
- **Spending** by top-level category over a date range (base currency).
- **Cash flow**: income vs. expense per month (base currency).

### Web Push / PWA
- Installable PWA (service worker, manifest, icons).
- **Web Push** notifications (VAPID): the browser subscribes for a badge on the
  uncategorized-transaction count; subscriptions are registered/removed
  server-side. Disabled automatically when VAPID keys aren't configured.

### Telegram userbot (`lookout`)
- Polls a single Telegram DM for Humo bank alerts, parses message text into
  structured records (pure, fixture-tested), **pairs two-sided transfers** into
  a single transaction, and delivers them to the ingest API with retries.
- Atomic JSON state (poll watermark + pending transfer legs) and balance-gap
  reconciliation. Read-only on Telegram, idempotent on the app side.

## The money core (`core/internal/ledger`)

Pure, unit-tested logic that everything else builds on — kept out of the HTTP
handlers:

- balance derivation from transactions
- net-worth aggregation across currencies
- `rate_to_base` freezing
- loan amortization
- reconciliation (reported vs. derived)
- transaction validation rules

## Project layout

```
specs/        OpenAPI contract (single source of truth)
core/         Go backend + REST API
  cmd/        server · migrate · seed · vapidkeys entrypoints
  config/     env config (cleanenv)
  internal/
    app/          fx wiring, HTTP router, lifecycle
    entities/     domain types (no I/O)
    ledger/       money core (pure, tested)
    repositories/ pgx data access, one pkg per aggregate
    http/         handlers + middleware (OpenAPI validation, auth)
    iconsuggest/  LLM icon suggestions
    integration/  black-box API tests against real Postgres
  generated/    oapi-codegen output (never edit)
  migrations/   numbered SQL up/down migrations
  pkg/          third-party wrappers (options pattern):
                anthropic · database · fx · httpserver · log ·
                migrator · money · webpush
frontend/     Vue 3 + Vite SPA (PWA)
lookout/      Telegram userbot
```

## Running locally

Prerequisites: Go, Node, and Docker (for Postgres).

```bash
# 1. Start Postgres
docker compose up -d

# 2. Backend (from core/)
cd core
make migrate      # apply migrations
make seed         # load dev seed data
make run          # generates code + runs the API server (:8080)

# 3. Frontend (from frontend/)
cd frontend
npm install
npm run dev       # generates the API client + starts Vite
```

### Useful `core/` make targets

| Target              | Purpose                                            |
|---------------------|----------------------------------------------------|
| `make generate`     | Regenerate Go code from `specs/`                   |
| `make run` / `build`| Run / build the API server                         |
| `make migrate`      | Apply migrations (`migrate-fresh` to reset)        |
| `make seed`         | Load dev seed data (`seed-reset` to wipe + reload) |
| `make vapid-keys`   | Generate VAPID keypair for Web Push                |
| `make test`         | Full test suite (`-race`)                          |
| `make lint`         | golangci-lint                                      |

### Configuration

Core is configured entirely via environment variables (see `core/config`).
Notable ones:

- `BASE_CURRENCY` — reporting base currency (default `UZS`).
- `AUTH_USERNAME` / `AUTH_PASSWORD` — Basic auth for the UI/API.
- `INGEST_TOKEN` — bearer token for the ingest endpoints.
- `ANTHROPIC_API_KEY` / `ANTHROPIC_MODEL` — enable LLM suggestions (optional).
- `VAPID_PUBLIC_KEY` / `VAPID_PRIVATE_KEY` / `VAPID_SUBSCRIBER` — enable Web
  Push (optional).
- `POSTGRES_*` — database connection.

## Testing

The money core is tested before any UI work. Run the backend suite with:

```bash
cd core && make test
```

`core/internal/integration` additionally runs black-box API tests against a real
Postgres instance.

## Deployment

Each service deploys as a separate app alongside a managed Postgres, served
single-origin (`/api` → core, everything else → frontend). See
[`DEPLOY.md`](./DEPLOY.md) for the full Dokploy setup.
