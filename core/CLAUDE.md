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