# CLAUDE.md

Project memory for Claude Code. Keep this short — full detail lives in **REQUIREMENTS.md**.

## What we're building
A private, multi-currency personal finance app: accounts, loans, transfers, and
income/expenses by category, with net-worth and spending reporting.
Build **Phase 1 (MVP) first**, in the order given in REQUIREMENTS.md §9.

## Stack
Golang · vue+vite · pgsql via pgx, https://github.com/Rhymond/go-money for money · suite

## Non-negotiable invariants (never violate)
- Money is stored as **integer minor units** + ISO-4217 currency code. Never floating point.
- Every non-base-currency transaction stores a **frozen `rate_to_base`**; reports use
  the stored rate, never a live re-conversion.
- **Transfers have no category and don't change net worth.** Expenses and income
  require a category whose `type` matches.
- **Net worth = Σ assets − Σ liabilities.** Credit cards and loans are liability accounts.
- A credit card is a liability account with no schedule (revolving); a loan is a
  liability account with schedule metadata.
- Accounts/categories that have transactions are **archived, never hard-deleted**.

## Workflow
- Migrations from day one; commit dev seed data.
- **Test the money core before building any UI:** balance derivation, net-worth
  aggregation across currencies, `rate_to_base` freezing, cross-currency transfers,
  loan amortization.
- Run the full test suite before treating any task as done.

folder structure:
/core - golang app, api
/frontend - vue app

For every third part package should be wrapper in pkg with options pattern

## File Reading
Use `Read` tool directly instead of shell commands for reading files.
Prefer `Read` over `cat`, `sed`, `head` in bash.

## Comments
Don't write long comments in code. only short ones

## Project Structure

Monorepo with three deployables — `core`, `frontend`, `lookout` — that integrate **only** through `specs/`.

### Top level
- `specs/` — OpenAPI contract, single source of truth. `api.yaml` (paths + schemas), `definitions.yaml` (shared enums). Drives Go codegen (core, lookout) and TS codegen (frontend).
- `core/` — Go backend + REST API; the system of record.
- `frontend/` — Vue 3 + Vite SPA.
- `lookout/` — Go Telegram userbot: parses bank notifications → posts to core's ingest API.

### core/ — Go (uber-fx DI · chi · pgx)
- `cmd/server/` — API server entrypoint (`main.go` boots the fx app).
- `cmd/migrate/` — golang-migrate runner (`up` / `fresh`).
- `cmd/seed/` — dev seed-data loader.
- `config/` — env config via cleanenv (`config.go` — all config structs + `RegisterConfigs` fx providers).
- `internal/app/` — fx wiring / DI graph, HTTP router, lifecycle (`app.go`).
- `internal/entities/` — domain types (account, category, category_rule, budget, transaction). No I/O.
- `internal/ledger/` — money core: balance derivation, net-worth, loan amortization, transaction rules. Pure + unit-tested.
- `internal/repositories/` — pgx data access, one pkg per aggregate. `category_repository` holds `ResolveForIngest` (merchant/card → category routing).
- `internal/http/handlers/` — REST handlers, one file per resource. `main.go` = `Server` implementing the generated `StrictServerInterface`.
- `internal/http/middlewares/` — OpenAPI request validation + auth (BasicAuth / IngestAuth).
- `internal/iconsuggest/` — LLM category-icon suggestions (Anthropic; disabled when no API key).
- `internal/integration/` — black-box API tests against a real Postgres.
- `generated/api/` — oapi-codegen output from `specs/`. **Generated — never edit; run `make generate`.**
- `migrations/` — numbered SQL up/down migrations.
- `pkg/` — third-party wrappers (options pattern): `anthropic`, `database` (pgx pool), `httpserver`, `log` (zap), `migrator`, `money` (go-money), `fx` (currency rates).
- `Makefile` — `generate` / `run` / `build` / `migrate` / `seed` / `test` / `lint`.

### frontend/ — Vue 3 (Vite · Tailwind · openapi-fetch)
- `src/api/` — typed API layer. `client.ts` (openapi-fetch + auth), `types.ts` (re-exported schema types), one module per resource; `schema.d.ts` is generated.
- `src/views/` — routed pages (Dashboard, Accounts, Transactions, Categories, Budgets, Login).
- `src/components/` — forms, modals, icon picker (`*Form`, `Modal`, `IconField`, `CategoryIcon`, `CategoryRulesManager`).
- `src/lib/` — helpers: `format.ts` (money/date), `tablerIcon.ts` + `tablerIconNames.ts` (icon webfont).
- `src/router/` — vue-router routes + auth guard. `src/main.ts` = entry, `App.vue` = shell/nav.
- `src/assets/tabler/` — vendored Tabler icon webfont (CSS + woff2). `public/` — static assets (favicon).
- `scripts/gen-api.mjs` — generates `src/api/schema.d.ts` from `../specs/api.yaml`.

### lookout/ — Go Telegram userbot
- `cmd/lookout/` — entrypoint. `internal/config/` — env config.
- `internal/telegram/` — gotd user session, peer resolve, history polling.
- `internal/parser/` — bank message text → structured record (pure, fixture-tested).
- `internal/pairing/` — transfer-leg buffer (collapses two-sided transfers).
- `internal/delivery/` — posts to core ingest via the generated client; retries.
- `internal/store/` — atomic JSON state (watermark + pending legs). `internal/recon/` — balance-gap detection.
- `internal/app/` — orchestrator: poll → parse → pair → deliver loop.
- `generated/core/` — ingest client generated from `specs/`. **Generated — never edit.**

## Navigation Rules
- **Read this map before opening any file.** Find the directory/file here first; don't grep blindly across the repo.
- Services integrate **only through `specs/`** — never import types across `core` / `frontend` / `lookout`. To change behavior, edit `specs/*.yaml`, then `make generate` in each affected service.
- `generated/` (Go) and `src/api/schema.d.ts` (TS) are codegen output — never hand-edit.
- Backend request flow: `specs` → `generated/api` (StrictServerInterface) → `internal/http/handlers` → `internal/repositories` → Postgres. Money logic lives in `internal/ledger`, not handlers.
