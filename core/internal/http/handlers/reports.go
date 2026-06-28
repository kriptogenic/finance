package handlers

import (
	"context"
	"time"

	"github.com/oapi-codegen/nullable"

	"finance/generated/api"
	"finance/internal/ledger"
	"finance/pkg/money"
)

func (s Server) GetNetWorth(ctx context.Context, _ api.GetNetWorthRequestObject) (api.GetNetWorthResponseObject, error) {
	accounts, err := s.accounts.List(ctx, false)
	if err != nil {
		return nil, err
	}

	balances, err := s.accounts.Balances(ctx)
	if err != nil {
		return nil, err
	}

	rates, err := s.reports.LatestRates(ctx)
	if err != nil {
		return nil, err
	}

	abs := make([]ledger.AccountBalance, 0, len(accounts))
	for _, a := range accounts {
		if a.ExcludedFromNetWorth {
			continue
		}
		abs = append(abs, ledger.AccountBalance{Account: a, Balance: balances[a.ID]})
	}

	nw := ledger.ComputeNetWorth(s.base, abs, rates)

	return api.GetNetWorth200JSONResponse(s.toNetWorth(nw)), nil
}

func (s Server) GetSpending(ctx context.Context, request api.GetSpendingRequestObject) (api.GetSpendingResponseObject, error) {
	from, to := dateRange(request.Params.DateFrom, request.Params.DateTo)

	spends, err := s.reports.SpendingByCategory(ctx, from, to)
	if err != nil {
		return nil, err
	}

	total := money.Zero(s.base)
	categories := make([]api.CategorySpend, len(spends))
	for i, sp := range spends {
		if total, err = total.Plus(sp.Amount); err != nil {
			return nil, err
		}
		categories[i] = api.CategorySpend{
			CategoryId:   sp.CategoryID,
			CategoryName: sp.CategoryName,
			Amount:       sp.Amount,
		}
	}

	return api.GetSpending200JSONResponse{
		Base:       s.base,
		Total:      total,
		Categories: categories,
	}, nil
}

func (s Server) GetCashFlow(ctx context.Context, request api.GetCashFlowRequestObject) (api.GetCashFlowResponseObject, error) {
	from, to := dateRange(request.Params.DateFrom, request.Params.DateTo)

	flows, err := s.reports.CashFlow(ctx, from, to)
	if err != nil {
		return nil, err
	}

	months := make([]api.MonthFlow, len(flows))
	for i, f := range flows {
		net, nerr := f.Income.Minus(f.Expense)
		if nerr != nil {
			return nil, nerr
		}
		months[i] = api.MonthFlow{
			Month:   f.Month,
			Income:  f.Income,
			Expense: f.Expense,
			Net:     net,
		}
	}

	return api.GetCashFlow200JSONResponse{Base: s.base, Months: months}, nil
}

func (s Server) GetForecast(ctx context.Context, request api.GetForecastRequestObject) (api.GetForecastResponseObject, error) {
	start := forecastMonth(request.Params.Month)
	end := start.AddDate(0, 1, 0)

	schedules, err := s.schedules.List(ctx)
	if err != nil {
		return nil, err
	}

	budgets, err := s.budgets.List(ctx)
	if err != nil {
		return nil, err
	}

	rates, err := s.reports.LatestRates(ctx)
	if err != nil {
		return nil, err
	}

	f := ledger.ForecastMonth(s.base, schedules, budgets, start, end, rates)

	missing := f.MissingRates
	if missing == nil {
		missing = []string{}
	}

	lines := make([]api.ForecastLine, len(f.Lines))
	for i, l := range f.Lines {
		line := api.ForecastLine{
			ScheduleId:  l.Schedule.ID,
			Type:        api.TransactionType(l.Schedule.Type),
			Amount:      l.Amount,
			Occurrences: l.Occurrences,
		}
		if l.Schedule.Name != nil {
			line.Name = nullable.NewNullableWithValue(*l.Schedule.Name)
		}
		if l.Schedule.CategoryID != nil {
			line.CategoryId = nullable.NewNullableWithValue(*l.Schedule.CategoryID)
		}
		if l.Schedule.FromAccountID != nil {
			line.FromAccountId = nullable.NewNullableWithValue(*l.Schedule.FromAccountID)
		}
		if l.Schedule.ToAccountID != nil {
			line.ToAccountId = nullable.NewNullableWithValue(*l.Schedule.ToAccountID)
		}
		lines[i] = line
	}

	budgetLines := make([]api.ForecastBudgetLine, len(f.BudgetLines))
	for i, b := range f.BudgetLines {
		budgetLines[i] = api.ForecastBudgetLine{
			BudgetId:   b.Budget.ID,
			CategoryId: b.Budget.CategoryID,
			Period:     api.BudgetPeriod(b.Budget.Period),
			Amount:     b.Amount,
		}
	}

	return api.GetForecast200JSONResponse{
		Base:         s.base,
		Month:        start.Format("2006-01"),
		Income:       f.Income,
		Expense:      f.Expense,
		Transfers:    f.Transfers,
		Net:          f.Net,
		Lines:        lines,
		BudgetLines:  budgetLines,
		MissingRates: missing,
	}, nil
}

// forecastMonth resolves the optional YYYY-MM query param to the first day of
// that month (UTC). An empty or unparseable value defaults to the current month.
func forecastMonth(month *string) time.Time {
	now := time.Now().UTC()
	if month != nil {
		if t, err := time.Parse("2006-01", *month); err == nil {
			return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
		}
	}

	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
}

func (s Server) toNetWorth(nw ledger.NetWorthBreakdown) api.NetWorthReport {
	out := api.NetWorthReport{
		Base:         s.base,
		NetWorth:     nw.Net,
		Assets:       nw.Assets,
		Liabilities:  nw.Liabilities,
		ByCurrency:   make([]api.CurrencyExposure, len(nw.ByCurrency)),
		MissingRates: nw.MissingRates,
	}
	if out.MissingRates == nil {
		out.MissingRates = []string{}
	}

	for i, e := range nw.ByCurrency {
		ce := api.CurrencyExposure{
			Currency:    e.Currency,
			Assets:      e.Assets,
			Liabilities: e.Liabilities,
			Net:         e.Net,
			RateKnown:   e.RateKnown,
		}
		if e.RateKnown {
			netInBase := e.NetInBase
			ce.NetInBase = &netInBase
		}
		out.ByCurrency[i] = ce
	}

	return out
}

// dateRange resolves optional bounds to a concrete [from, to] window. Omitted
// bounds default to "all time" up to now.
func dateRange(from, to *time.Time) (time.Time, time.Time) {
	start := time.Unix(0, 0)
	end := time.Now()
	if from != nil {
		start = *from
	}
	if to != nil {
		end = *to
	}

	return start, end
}
