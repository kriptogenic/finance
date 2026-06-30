package audit

import (
	"context"
	"time"

	"finance/internal/ledger"
	mcpwrap "finance/pkg/mcp"
	"finance/pkg/money"
)

type forecastIn struct {
	Month string `json:"month,omitempty" jsonschema:"target month YYYY-MM; empty for the current month"`
}

type forecastOut struct {
	Base         string      `json:"base"`
	Month        string      `json:"month"`
	Income       money.Money `json:"income"`
	Expense      money.Money `json:"expense"`
	Net          money.Money `json:"net"`
	MissingRates []string    `json:"missing_rates"`
}

func (s *Service) forecast(ctx context.Context, _ *mcpwrap.CallToolRequest, in forecastIn) (*mcpwrap.CallToolResult, any, error) {
	start := forecastMonth(in.Month)
	end := start.AddDate(0, 1, 0)

	schedules, err := s.schedules.List(ctx)
	if err != nil {
		return nil, nil, err
	}
	budgets, err := s.budgets.List(ctx)
	if err != nil {
		return nil, nil, err
	}
	cardSpend, err := s.reports.CreditCardSpend(ctx, start, end)
	if err != nil {
		return nil, nil, err
	}
	rates, err := s.reports.LatestRates(ctx)
	if err != nil {
		return nil, nil, err
	}

	cardUsage := make([]ledger.CreditCardUsage, len(cardSpend))
	for i, c := range cardSpend {
		cardUsage[i] = ledger.CreditCardUsage{AccountID: c.AccountID, Amount: c.Amount}
	}

	f := ledger.ForecastMonth(s.base, schedules, budgets, cardUsage, start, end, rates)
	out := forecastOut{
		Base:         s.base,
		Month:        start.Format("2006-01"),
		Income:       f.Income,
		Expense:      f.Expense,
		Net:          f.Net,
		MissingRates: f.MissingRates,
	}
	if out.MissingRates == nil {
		out.MissingRates = []string{}
	}

	return result(out)
}

// forecastMonth resolves an optional YYYY-MM to the first day of that month
// (UTC), defaulting to the current month (mirrors handlers.forecastMonth).
func forecastMonth(month string) time.Time {
	now := time.Now().UTC()
	if month != "" {
		if t, err := time.Parse("2006-01", month); err == nil {
			return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
		}
	}

	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
}
