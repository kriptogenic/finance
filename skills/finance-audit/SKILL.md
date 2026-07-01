---
name: "finance-audit"
description: "Run a personal-finance audit using the `finance` MCP server: review net worth, cash flow, spending vs budgets, forecast, and data hygiene, then save a structured audit report. Use when the user asks to audit, review, or check their finances, spending, or budgets."
---

You are auditing the user's personal finances through the **`finance` MCP server**. Every data tool is read-only; the only write is `save_audit_report`. Never claim to have changed a transaction, account, or balance — you cannot.

## 0. Preconditions

Confirm the `finance` tools are available (`net_worth`, `cash_flow`, `spending_by_category`, `list_transactions`, `list_budgets`, `forecast`, `save_audit_report`, `list_audit_reports`). If they are missing, stop and tell the user:
> The `finance` connector isn't reachable. Make sure the core server is running (`make run` in `core/`, serving `/mcp` on :8080) and the finance connector is enabled, then reopen this chat.

## 1. Scope the audit

Pick the period to audit:
- Default to the **last 3 full calendar months** (exclude the current partial month).
- If the user named a period ("this year", "Q1", "last month"), use that.
- Express bounds as `YYYY-MM-DD`. Note today's date from context to compute them.

State the period you're auditing in one line before pulling data.

## 2. Gather data

Call these tools (dates as `YYYY-MM-DD`):
1. `net_worth` — current assets, liabilities, net, per-currency exposure, `missing_rates`.
2. `cash_flow` with `date_from`/`date_to` — monthly income vs expense vs net.
3. `spending_by_category` with the same range — expense per top-level category.
4. `list_budgets` — configured per-category limits.
5. `forecast` with the current `month` (`YYYY-MM`) — projected income/expense/net.
6. `list_transactions` to spot-check anomalies, e.g.:
   - `{"uncategorized": true, "limit": 50}` — data-hygiene gaps.
   - `{"date_from": ..., "date_to": ..., "limit": 200}` — scan for unusually large amounts vs the category norm.

Pull only what you need; you don't have to call every tool if the user asked something narrow.

## 3. Analyze

Work through this checklist and turn anything notable into a **finding**:

- **Net worth & liabilities** — Is net worth positive? How large are liabilities (credit cards, loans) vs assets? Any single liability dominating?
- **Savings rate** — From `cash_flow`, compute net ÷ income per month and the trend. Flag months where expense ≥ income (negative net).
- **Budget adherence** — For each budget, compare its limit to actual `spending_by_category`. Flag categories over budget; note categories with large spend and **no** budget.
- **Spending concentration** — Which 2–3 categories dominate? Any big month-over-month jump?
- **Forecast** — Does the projected month's net go negative? Is planned expense out of line with recent actuals?
- **Data hygiene** — Count uncategorized transactions; flag if non-trivial. Note duplicate-looking or suspiciously large transactions.
- **Currency exposure** — If `net_worth.missing_rates` is non-empty, flag that those currencies are excluded from base totals (numbers understate reality).

## 4. Severity

Tag each finding:
- `critical` — negative/declining net worth, spending consistently above income, a budget blown by a wide margin.
- `warning` — over budget, low/erratic savings rate, rising liabilities, missing FX rates.
- `info` — data hygiene, minor observations, things to keep watching.

## 5. Present, then save

1. Show the user a short markdown summary: the period, headline numbers (net worth, avg monthly net, savings rate), and the findings grouped by severity with concrete numbers.
2. Then call **`save_audit_report`**:
   ```json
   {
     "title": "Finance audit — <period>",
     "period_from": "YYYY-MM-DD",
     "period_to": "YYYY-MM-DD",
     "summary": "<the markdown summary>",
     "findings": [
       {"severity": "warning", "category": "spending", "message": "Dining was 32% over budget (…)"},
       {"severity": "info", "category": "data", "message": "7 uncategorized transactions"}
     ]
   }
   ```
3. Confirm the saved report's `id` to the user. Use `list_audit_reports` if they want to see past audits.

If the user explicitly says not to save, skip step 2 and say so.

## Money formatting

Tool amounts are **integer minor units** with a `currency` code, e.g. `{"amount": 1663750000, "currency": "UZS"}`. Divide by 100 to get the major value (`16,637,500.00 UZS`) and always show the currency. The base reporting currency is whatever `net_worth.base` reports (default `UZS`). Never invent exchange rates — if a currency is in `missing_rates`, say its base-currency value is unknown.
