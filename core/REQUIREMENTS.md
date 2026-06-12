# Personal Finance Manager — Requirements

> Spec for building a personal finance management app. Written to be handed to
> Claude Code. Build incrementally per the phased scope in §8.

---

## 1. Overview

A single-user personal finance app that tracks **accounts, loans, transfers, and
income/expenses across multiple currencies**, and reports on net worth, spending,
and cash flow.

Design priorities, in order: **correctness of money math**, privacy/local-first,
fast data entry, clear reporting.

---

## 2. Core concept — the "bucket" model

Every transaction moves money between two **buckets**. Accounts and categories are
both buckets. This one idea unifies all transaction types:

| Transaction | Moves from | Moves to |
|---|---|---|
| Expense | account | expense category |
| Income | income category (source) | account |
| Transfer | account | account |
| Take a loan | loan account | asset account |
| Repay a loan | asset account | loan account (+ interest as an expense) |

Accounts split into two kinds:

- **Asset accounts** (cash, debit card, deposit) — balance = what you own.
- **Liability accounts** (credit card, loan) — balance = what you owe.

A **credit card is a liability account with no schedule** (revolving), optionally
with a credit limit. A **loan is a liability account with schedule metadata**
(rate, term, amortization). Paying a card bill or a loan installment is just a
transfer from an asset account into the liability account.

**Net worth (base currency) = Σ(asset balances) − Σ(liability balances).**

---

## 3. Money & currency rules (read first — these prevent bugs)

1. **Store money as integer minor units** (e.g. cents) plus an ISO-4217 currency
   code. Never use floating point for amounts. Use a decimal/money library for
   arithmetic.
2. Each account holds exactly **one currency**.
3. Pick one **base/reporting currency** (user setting). All cross-account totals
   and net worth are computed in the base currency.
4. **Freeze the FX rate at transaction time.** Every transaction in a non-base
   currency stores its own `rate_to_base` (and a derived `base_amount`). Reports
   must use the stored rate, never a live re-conversion — otherwise historical
   reports silently change when rates move.
5. Account **current balances** convert to base at the latest known rate. For
   **net-worth-over-time** charts, convert each period's balance at that period's
   end-of-period rate.

---

## 4. Data model

Storage is relational. Subtype-specific fields below may be implemented as
nullable columns, a JSON column, or separate detail tables — implementer's choice.

### Account
| Field | Type | Notes |
|---|---|---|
| id | id | |
| name | string | |
| kind | enum | `asset` \| `liability` |
| type | enum | `cash` \| `debit_card` \| `deposit` \| `credit_card` \| `loan` |
| currency | string | ISO 4217 |
| opening_balance | integer (minor units) | for liabilities, positive = amount owed at start |
| archived | bool | soft-delete flag |
| created_at | datetime | |

Subtype fields (nullable unless the type applies):
- **deposit**: `interest_rate`, `term_months`, `maturity_date`, `capitalization` (bool/enum)
- **credit_card**: `credit_limit`
- **loan**: `principal`, `interest_rate`, `start_date`, `term_months`, `payment_day`

### Category
| Field | Type | Notes |
|---|---|---|
| id | id | |
| name | string | |
| parent_id | id? | self-reference; null = top level. Recommend 2 levels (category → subcategory) |
| type | enum | `expense` \| `income` (the tree has two roots) |
| icon | string? | |
| color | string? | |
| archived | bool | |

### Transaction
| Field | Type | Notes |
|---|---|---|
| id | id | |
| date | datetime | |
| type | enum | `expense` \| `income` \| `transfer` |
| from_account_id | id? | |
| to_account_id | id? | |
| category_id | id? | |
| amount | integer (minor units) | primary leg, in its account's currency |
| currency | string | |
| to_amount | integer? | transfers only; in `to_account` currency |
| to_currency | string? | transfers only |
| rate_to_base | decimal? | required when currency ≠ base |
| base_amount | integer? | derived: `amount × rate_to_base` |
| note | string? | |
| tags | string[] | |

**Field applicability by type:**

| Field | expense | income | transfer |
|---|---|---|---|
| from_account_id | required | — | required |
| to_account_id | — | required | required |
| category_id | required | required | — (must be null) |
| amount / currency | from account | to account | from-leg |
| to_amount / to_currency | — | — | required if cross-currency |

---

## 5. Business rules & invariants

- **Balance is derived**, not stored as a mutable field:
  `account.balance = opening_balance + Σ(inflows) − Σ(outflows)` in account currency.
  (A cached balance is fine as long as it's recomputable.)
- A **transfer never has a category** and **does not change net worth** — it only
  moves value between two accounts.
- An **expense/income must have a category**; the category's `type` must match the
  transaction type.
- Any transaction whose `currency ≠ base` **must** have `rate_to_base`.
- **Liability balances reduce net worth** automatically (see §2).
- **Deletion:** if an account or category has transactions, only allow
  **soft-archive**; hard-delete only when it has none.
- A **credit card may go negative** (you owe); enforce/ warn against exceeding
  `credit_limit` if set.
- Cross-currency transfer: `amount` (from currency) and `to_amount` (to currency)
  are independent; the implied rate is `to_amount / amount`.

---

## 6. Functional requirements

**Accounts** — CRUD; group by kind (assets / liabilities); show per-account balance
and base-currency equivalent; archive instead of delete when in use.

**Categories** — CRUD with one level of subcategories; separate expense and income
trees; reassign/merge on archive.

**Transactions** — add/edit/delete expense, income, transfer; cross-currency
transfer support; search and filter (by account, category, date range, tag, text);
optional notes and tags.

**Loans** — store loan terms; generate and display an **amortization schedule**;
track payoff progress and total interest paid; record installments as transfers
(principal) + expense (interest).

**Reporting / dashboard** — net worth (with assets vs liabilities breakdown);
spending by category over a chosen period; cash flow (income vs expense per month);
currency-exposure breakdown (how much net worth sits in each currency).

---

## 7. Non-functional requirements

- **Local-first / privacy**: financial data stays on the user's device or
  self-hosted store by default; no third-party data sharing.
- **Security**: app lock (PIN/biometric); encrypt the data store and backups.
- **Backup / portability**: export and import a full JSON backup; export
  transactions to CSV.
- **Offline**: fully usable without a network connection.
- **Performance**: snappy with tens of thousands of transactions.
- **Correctness**: money and currency logic must be unit-tested (see §9).

---

## 8. Scope & phasing

**Phase 1 — MVP**
- Accounts (cash, debit card, deposit, credit card, loan) with asset/liability split
- Transactions: expense, income, transfer (incl. cross-currency, manual rates)
- Categories + subcategories (expense & income trees)
- Derived balances + **net worth**
- Base-currency setting; `rate_to_base` per transaction
- Basic list/search; JSON + CSV export

**Phase 2 — make it "alive"**
- **Recurring / scheduled transactions** (salary, rent, subscriptions)
- **Per-category budgets** with progress and overspend alerts
- **Dashboard**: net worth over time, spending by category, cash flow
- Loan amortization schedules + payoff progress
- App lock + encrypted backup

**Phase 3 — power features**
- CSV/bank-statement import + auto-categorization rules
- Auto-fetch daily FX rates
- Savings goals, bill reminders
- Receipt/attachment support, Payee entity, top-merchant reports
- Multi-user / shared household accounts

Additional Phase 2/3 entities (introduce only when their phase begins):
`RecurringRule`, `Budget`, `ExchangeRate`, `Payee`, `Attachment`.

---

## 9. Notes for Claude Code

Golang · vue+vite · pgsql via pgx, https://github.com/Rhymond/go-money for money · suite

**Build order:** (1) data layer + migrations + seed data → (2) account & category
CRUD → (3) transaction engine (the three types + cross-currency) → (4) derived
balances & net worth → (5) reporting → then Phase 2.

**Testing is mandatory for the money core.** Before building UI on top, write tests
for: balance derivation, net-worth aggregation across currencies, `rate_to_base`
freezing, cross-currency transfers, and the loan amortization schedule. These are
the parts where silent bugs cost real money.

**Keep transfers and categories mutually exclusive** in both schema constraints and
UI — it's the most common modeling mistake in finance apps.

---

## Appendix A — Phase 2 entities (detailed)

Introduce each only when Phase 2 begins. All money fields follow §3 (integer minor
units + currency code).

### RecurringRule
Auto-generates transactions on a schedule (salary, rent, subscriptions).

| Field | Type | Notes |
|---|---|---|
| id | id | |
| name | string | label, e.g. "Salary" |
| enabled | bool | |
| type | enum | mirrors Transaction: `expense` / `income` / `transfer` |
| from_account_id | id? | follows the §4 per-type rules |
| to_account_id | id? | |
| category_id | id? | |
| amount | integer | template amount, in primary-leg currency |
| currency | string | |
| to_amount / to_currency | integer? / string? | transfers, cross-currency |
| note | string? | |
| tags | string[] | |
| frequency | enum | `daily` / `weekly` / `monthly` / `yearly` |
| interval | integer | every N periods (e.g. 2 = biweekly) |
| anchor | string? | day-of-month or weekday the rule fires on |
| start_date | date | |
| end_date | date? | null = open-ended |
| posting_mode | enum | `auto` (post silently) or `confirm` (queue a pending item the user approves) |
| last_run | date? | |
| next_run | date | computed |

Rules:
- On generation, copy the template fields, stamp the **actual occurrence date**, and
  look up a **fresh `rate_to_base` for that date** — never reuse an old rate.
- Generated transactions carry `recurring_rule_id` (additive field on Transaction)
  so they can be traced, bulk-edited, or detached.
- Editing a rule must **not** retroactively change already-posted transactions.
- **Catch-up:** if the app was closed across due dates, on next open generate all
  missed occurrences up to today (auto mode) or queue them for confirmation.

### Budget
A spending limit for an expense category over a period.

| Field | Type | Notes |
|---|---|---|
| id | id | |
| category_id | id | expense category only; covers its subcategories too |
| period | enum | `weekly` / `monthly` / `yearly` (default monthly) |
| amount | integer | limit, in **base currency** |
| rollover | bool | unused amount carries to next period (optional) |
| start_period | date? | null = always active |

Rules:
- Spent = Σ `base_amount` of **expense** transactions in the category **and its
  subcategories** within the period.
- Income and transfers are excluded.
- Expose progress (spent / amount) and alert thresholds (e.g. 80%, 100%).

### ExchangeRate
A rate cache/history. **Suggestion source only** — never the source of truth for a
posted transaction (each transaction keeps its own frozen `rate_to_base`, §3).

| Field | Type | Notes |
|---|---|---|
| id | id | |
| date | date | |
| quote_currency | string | the "from" currency |
| base_currency | string | the reporting currency |
| rate | decimal | units of base per 1 quote — document the direction and keep it consistent |
| source | string | `manual` or provider name |

Uses & rules:
- Unique on `(date, quote_currency, base_currency)`.
- When entering a transaction, prefill `rate_to_base` from the row matching its date;
  the user can override. The stored transaction snapshot is authoritative thereafter.
- For **net-worth-over-time**, convert each period's balances using that period's
  end-of-period rate from this table.
- Phase 2: manual entry. Phase 3: auto-fetch populates it.

**Additive change to existing schema:** add nullable `recurring_rule_id` to
`Transaction` when RecurringRule lands.
