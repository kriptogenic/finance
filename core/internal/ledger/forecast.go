package ledger

import (
	"sort"
	"time"

	"finance/internal/entities"
	"finance/pkg/fx"
	"finance/pkg/money"
)

// ForecastLine is one schedule's projected contribution to a month, in base
// currency: the primary-leg amount converted to base, times the occurrence count.
type ForecastLine struct {
	Schedule    entities.ScheduledTransaction
	Occurrences int
	Amount      money.Money
}

// BudgetLine is a budget's contribution to the forecast month: its limit
// normalized to a monthly equivalent, in base currency.
type BudgetLine struct {
	Budget entities.Budget
	Amount money.Money
}

// Forecast is the projected cash flow for a single month, in base currency.
// Transfers count as planned outflow (loan / credit-card payments) and budgets
// count as planned spending on top of scheduled expenses, so
// Net = Income − Expense − Transfers and Expense already includes the budgets.
type Forecast struct {
	Income       money.Money
	Expense      money.Money // scheduled expenses + budgets
	Transfers    money.Money
	Net          money.Money
	Lines        []ForecastLine // schedule-derived
	BudgetLines  []BudgetLine   // budget-derived (also summed into Expense)
	MissingRates []string       // currencies with no frozen/known rate; their lines are skipped
}

// Occurrences counts how many times s falls within the half-open window
// [start, end). A paused schedule never occurs. Steps from StartDate by the
// recurrence rule, stopping once EndDate (inclusive) is passed.
func Occurrences(s entities.ScheduledTransaction, start, end time.Time) int {
	if s.Paused {
		return 0
	}

	count := 0
	for cur := s.StartDate; cur.Before(end); cur = entities.NextRun(s.Frequency, s.Interval, cur) {
		if s.EndDate != nil && cur.After(*s.EndDate) {
			break
		}
		if !cur.Before(start) {
			count++
		}
	}

	return count
}

// ForecastMonth projects each schedule over [start, end) and aggregates the
// per-type totals in base currency, mirroring ComputeNetWorth's rate handling:
// an amount already in base is taken as-is, otherwise the schedule's frozen
// RateToBase wins, then the latest known rate; a currency with no rate is
// recorded in MissingRates and its line dropped.
func ForecastMonth(base string, schedules []entities.ScheduledTransaction, budgets []entities.Budget, start, end time.Time, rates map[string]fx.Rate) Forecast {
	var income, expense, transfers int64
	var lines []ForecastLine
	missing := map[string]bool{}

	for _, s := range schedules {
		n := Occurrences(s, start, end)
		if n == 0 {
			continue
		}

		baseMinor, ok := toBaseMinor(s.Amount, s.RateToBase, base, rates)
		if !ok {
			missing[s.Amount.Code()] = true

			continue
		}
		total := baseMinor * int64(n)

		lines = append(lines, ForecastLine{Schedule: s, Occurrences: n, Amount: money.New(total, base)})

		switch s.Type {
		case entities.TxIncome:
			income += total
		case entities.TxExpense:
			expense += total
		case entities.TxTransfer:
			transfers += total
		}
	}

	var budgetLines []BudgetLine
	for _, b := range budgets {
		// a budget that hasn't started by the month's end doesn't apply yet
		if b.StartPeriod != nil && !b.StartPeriod.Before(end) {
			continue
		}
		monthly := budgetMonthlyMinor(b)
		expense += monthly
		budgetLines = append(budgetLines, BudgetLine{Budget: b, Amount: money.New(monthly, base)})
	}

	f := Forecast{
		Income:      money.New(income, base),
		Expense:     money.New(expense, base),
		Transfers:   money.New(transfers, base),
		Net:         money.New(income-expense-transfers, base),
		Lines:       lines,
		BudgetLines: budgetLines,
	}
	for c := range missing {
		f.MissingRates = append(f.MissingRates, c)
	}
	sort.Strings(f.MissingRates)

	return f
}

// budgetMonthlyMinor normalizes a budget limit (already in base) to a monthly
// equivalent: weekly annualized then /12, yearly /12, monthly as-is.
func budgetMonthlyMinor(b entities.Budget) int64 {
	a := b.Amount.Minor()
	switch b.Period {
	case entities.BudgetWeekly:
		return a * 52 / 12
	case entities.BudgetYearly:
		return a / 12
	default: // monthly
		return a
	}
}

// toBaseMinor converts amount to base minor units. Returns false when a
// non-base currency has neither a frozen nor a latest rate.
func toBaseMinor(amount money.Money, frozen *fx.Rate, base string, rates map[string]fx.Rate) (int64, bool) {
	if amount.Code() == base {
		return amount.Minor(), true
	}
	if frozen != nil {
		return FreezeBase(amount, *frozen, base).Minor(), true
	}
	if rate, ok := rates[amount.Code()]; ok {
		return FreezeBase(amount, rate, base).Minor(), true
	}

	return 0, false
}
