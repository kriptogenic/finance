# Feature Ideas

Based on what's already built, grouped by how much they lean on existing
primitives. Prioritized toward filling visible gaps.

## High-leverage (build directly on existing primitives)

- **Recurring / scheduled transactions** — salary, rent, subscriptions. You
  already have transactions + categories; add a schedule table and a worker that
  materializes due ones. Natural fit with the loan `payment_day` concept.
- **Budget alerts via push** — Web Push is already wired for the uncategorized
  badge. Reuse it to notify when a budget crosses 80% / 100%. Very small addition
  for real value.
- **Net-worth over time (trend chart)** — you have `balance_snapshots`
  (currently only for reconciliation) and derived balances. Persist a daily
  net-worth point and chart it. Today all reports are point-in-time except
  cash-flow.
- **Loan payment tracking** — you compute the amortization *schedule* but don't
  track actual payments against it. Link transfers/expenses to a loan and show
  "ahead/behind schedule," remaining principal, early-payoff impact.
- **Deposit interest accrual** — you store `interest_rate` / `capitalization` /
  `maturity_date` on deposits but don't project or post interest. Add projected
  maturity value and optional auto-posting.

## Reporting & insight

- **Spending trends & comparisons** — month-over-month per category, "you spent
  X% more on Groceries than last month."
- **Forecasting / cash-flow projection** — combine recurring transactions +
  current balances to project balances forward.
- **Tag analytics** — tags exist on transactions but there's no report that
  slices by tag (e.g. "vacation 2026" total across categories).
- **Income vs. expense by category over time**, not just the flat spending
  report.

## Transaction power features

- **Split transactions** — one purchase divided across multiple categories
  (common gap; today a transaction has a single category).
- **Receipt/attachment storage** — attach an image/PDF to a transaction.
- **Bulk re-categorize** — select many uncategorized transactions and apply a
  category or create a rule in one action (pairs well with existing rules +
  uncategorized filter).
- **CSV / JSON export** (and import) — for backup and taxes.

## Automation (lookout & ingest)

- **More bank parsers** — lookout currently parses Humo. Adding other banks is
  isolated work in `lookout/internal/parser` with fixtures.
- **Auto-categorize on ingest using the LLM**, not just suggest — with a
  confidence threshold, falling back to the uncategorized bucket.

## UX / PWA

- **Offline transaction entry** — queue in the service worker, sync when back
  online (you already have a PWA + service worker).
- **Quick-add from the home screen / share target** — PWA share target to
  capture amounts fast.
- **Multi-currency display toggle** — show any account in base currency inline.

## Worth doing first

- You have a `pkg/fx` (currency rates) wrapper but transactions require a
  **manually entered `rate_to_base`**. **Auto-fetching the rate** at transaction
  time (with manual override) is probably the single highest-value small feature.
- **Savings goals** ("save 10M UZS by December") would be a genuinely new entity
  but fits the budget UI pattern.
