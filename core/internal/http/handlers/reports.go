package handlers

import (
	"context"
	"time"

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

	var total int64
	categories := make([]api.CategorySpend, len(spends))
	for i, sp := range spends {
		total += sp.Amount
		categories[i] = api.CategorySpend{
			CategoryId:   sp.CategoryID,
			CategoryName: sp.CategoryName,
			Amount:       money.New(sp.Amount, s.base),
		}
	}

	return api.GetSpending200JSONResponse{
		Base:       s.base,
		Total:      money.New(total, s.base),
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
		months[i] = api.MonthFlow{
			Month:   f.Month,
			Income:  money.New(f.Income, s.base),
			Expense: money.New(f.Expense, s.base),
			Net:     money.New(f.Income-f.Expense, s.base),
		}
	}

	return api.GetCashFlow200JSONResponse{Base: s.base, Months: months}, nil
}

func (s Server) toNetWorth(nw ledger.NetWorthBreakdown) api.NetWorthReport {
	out := api.NetWorthReport{
		Base:         s.base,
		NetWorth:     money.New(nw.Net, s.base),
		Assets:       money.New(nw.Assets, s.base),
		Liabilities:  money.New(nw.Liabilities, s.base),
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
			m := money.New(e.NetInBase, s.base)
			ce.NetInBase = &m
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
