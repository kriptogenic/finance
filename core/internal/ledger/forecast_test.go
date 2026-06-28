package ledger_test

import (
	"time"

	"finance/internal/entities"
	"finance/internal/ledger"
	"finance/pkg/fx"
	"finance/pkg/money"
)

func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func monthly(start time.Time, amount int64, ccy string, typ entities.TransactionType) entities.ScheduledTransaction {
	return entities.ScheduledTransaction{
		Type: typ, Amount: money.New(amount, ccy), Frequency: entities.FreqMonthly, Interval: 1, StartDate: start,
	}
}

func (s *LedgerSuite) TestOccurrences_CountsWithinWindow() {
	start := day(2026, time.June, 1)
	end := start.AddDate(0, 1, 0) // [Jun 1, Jul 1)

	monthlySc := monthly(day(2026, time.June, 1), 1, "USD", entities.TxExpense)
	s.Equal(1, ledger.Occurrences(monthlySc, start, end))

	weekly := entities.ScheduledTransaction{Frequency: entities.FreqWeekly, Interval: 1, StartDate: day(2026, time.June, 1)}
	s.Equal(5, ledger.Occurrences(weekly, start, end)) // Jun 1,8,15,22,29
}

func (s *LedgerSuite) TestOccurrences_OneTime() {
	start := day(2026, time.June, 1)
	end := start.AddDate(0, 1, 0) // [Jun 1, Jul 1)

	inMonth := entities.ScheduledTransaction{Frequency: entities.FreqOnce, StartDate: day(2026, time.June, 20)}
	s.Equal(1, ledger.Occurrences(inMonth, start, end))

	otherMonth := entities.ScheduledTransaction{Frequency: entities.FreqOnce, StartDate: day(2026, time.July, 20)}
	s.Equal(0, ledger.Occurrences(otherMonth, start, end))

	paused := entities.ScheduledTransaction{Frequency: entities.FreqOnce, StartDate: day(2026, time.June, 20), Paused: true}
	s.Equal(0, ledger.Occurrences(paused, start, end))
}

func (s *LedgerSuite) TestOccurrences_PausedAndEnded() {
	start := day(2026, time.June, 1)
	end := start.AddDate(0, 1, 0)

	paused := monthly(day(2026, time.June, 1), 1, "USD", entities.TxExpense)
	paused.Paused = true
	s.Equal(0, ledger.Occurrences(paused, start, end))

	ended := monthly(day(2026, time.January, 1), 1, "USD", entities.TxExpense)
	ended.EndDate = ptr(day(2026, time.March, 15))
	s.Equal(0, ledger.Occurrences(ended, start, end))

	future := monthly(day(2026, time.August, 1), 1, "USD", entities.TxExpense)
	s.Equal(0, ledger.Occurrences(future, start, end))
}

func (s *LedgerSuite) TestForecastMonth_AggregatesAndConverts() {
	start := day(2026, time.June, 1)
	end := start.AddDate(0, 1, 0)

	salary := monthly(start, 500000, "USD", entities.TxIncome)
	rent := monthly(start, 150000, "USD", entities.TxExpense)
	topUp := monthly(start, 100000, "USD", entities.TxTransfer)

	// EUR expense with a frozen rate (×2) → 60000 in base, independent of rates map.
	eurRent := monthly(start, 30000, "EUR", entities.TxExpense)
	eurRent.RateToBase = ptr(fx.MustParseRate("2"))

	f := ledger.ForecastMonth("USD",
		[]entities.ScheduledTransaction{salary, rent, topUp, eurRent}, nil,
		start, end, map[string]fx.Rate{})

	s.Equal(int64(500000), f.Income.Minor())
	s.Equal(int64(310000), f.Expense.Minor()) // 150000 expense + 60000 eur + 100000 transfer
	s.Equal(int64(190000), f.Net.Minor())     // 500000 - 310000
	s.Empty(f.MissingRates)
}

func (s *LedgerSuite) TestForecastMonth_FoldsBudgets() {
	start := day(2026, time.June, 1)
	end := start.AddDate(0, 1, 0)

	salary := monthly(start, 500000, "USD", entities.TxIncome)

	budgets := []entities.Budget{
		{Period: entities.BudgetMonthly, Amount: money.New(80000, "USD")},
		{Period: entities.BudgetWeekly, Amount: money.New(3000, "USD")},   // 3000*52/12 = 13000
		{Period: entities.BudgetYearly, Amount: money.New(120000, "USD")}, // 120000/12 = 10000
	}

	f := ledger.ForecastMonth("USD", []entities.ScheduledTransaction{salary}, budgets,
		start, end, map[string]fx.Rate{})

	s.Equal(int64(103000), f.Expense.Minor()) // 80000 + 13000 + 10000
	s.Equal(int64(397000), f.Net.Minor())     // 500000 - 103000
	s.Len(f.BudgetLines, 3)
}

func (s *LedgerSuite) TestForecastMonth_SkipsFutureBudget() {
	start := day(2026, time.June, 1)
	end := start.AddDate(0, 1, 0)

	future := entities.Budget{
		Period: entities.BudgetMonthly, Amount: money.New(80000, "USD"),
		StartPeriod: ptr(day(2026, time.August, 1)),
	}

	f := ledger.ForecastMonth("USD", nil, []entities.Budget{future}, start, end, map[string]fx.Rate{})

	s.Equal(int64(0), f.Expense.Minor())
	s.Empty(f.BudgetLines)
}

func (s *LedgerSuite) TestForecastMonth_MissingRateSkipsLine() {
	start := day(2026, time.June, 1)
	end := start.AddDate(0, 1, 0)

	eur := monthly(start, 30000, "EUR", entities.TxExpense) // no frozen rate, none in map

	f := ledger.ForecastMonth("USD", []entities.ScheduledTransaction{eur}, nil, start, end, map[string]fx.Rate{})

	s.Equal(int64(0), f.Expense.Minor())
	s.Empty(f.Lines)
	s.Equal([]string{"EUR"}, f.MissingRates)
}
