package ledger

import (
	"finance/pkg/fx"
	"finance/pkg/money"
)

// CurrencyExposure is the asset/liability/net position held in one currency,
// plus its base-currency conversion when a rate is known (§6 currency exposure).
type CurrencyExposure struct {
	Currency    string
	Assets      money.Money // in Currency
	Liabilities money.Money
	Net         money.Money
	NetInBase   money.Money // zero when RateKnown is false
	RateKnown   bool
}

// NetWorthBreakdown is the net-worth report: base totals plus a per-currency
// breakdown. Currencies with no known rate are listed in MissingRates and left
// out of the base totals.
type NetWorthBreakdown struct {
	Base         string
	Assets       money.Money // in Base
	Liabilities  money.Money
	Net          money.Money
	ByCurrency   []CurrencyExposure
	MissingRates []string
}

// ComputeNetWorth groups balances by currency, converts each currency's
// position to base using the latest known rate (identity for base itself), and
// returns the breakdown. Net is kept as Assets − Liabilities for internal
// consistency regardless of per-currency rounding.
func ComputeNetWorth(base string, balances []AccountBalance, rates map[string]fx.Rate) NetWorthBreakdown {
	type position struct{ assets, liabilities int64 }

	positions := map[string]*position{}
	order := make([]string, 0)

	for _, ab := range balances {
		cur := ab.Account.Currency
		p, ok := positions[cur]
		if !ok {
			p = &position{}
			positions[cur] = p
			order = append(order, cur)
		}

		if ab.Account.IsLiability() {
			p.liabilities += ab.Balance.Minor()
		} else {
			p.assets += ab.Balance.Minor()
		}
	}

	out := NetWorthBreakdown{Base: base}
	assets, liabilities := int64(0), int64(0)

	for _, cur := range order {
		p := positions[cur]
		exposure := CurrencyExposure{
			Currency:    cur,
			Assets:      money.New(p.assets, cur),
			Liabilities: money.New(p.liabilities, cur),
			Net:         money.New(p.assets-p.liabilities, cur),
			NetInBase:   money.Zero(base),
		}

		rate, known := rateToBase(cur, base, rates)
		if known {
			exposure.RateKnown = true
			exposure.NetInBase = money.New(rate.Convert(p.assets-p.liabilities), base)
			assets += rate.Convert(p.assets)
			liabilities += rate.Convert(p.liabilities)
		} else {
			out.MissingRates = append(out.MissingRates, cur)
		}

		out.ByCurrency = append(out.ByCurrency, exposure)
	}

	out.Assets = money.New(assets, base)
	out.Liabilities = money.New(liabilities, base)
	out.Net = money.New(assets-liabilities, base)

	return out
}

func rateToBase(currency, base string, rates map[string]fx.Rate) (fx.Rate, bool) {
	if currency == base {
		return fx.One(), true
	}

	if r, ok := rates[currency]; ok && r.Valid() {
		return r, true
	}

	return fx.Rate{}, false
}
