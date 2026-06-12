package ledger

import "finance/pkg/fx"

// CurrencyExposure is the asset/liability/net position held in one currency,
// plus its base-currency conversion when a rate is known (§6 currency exposure).
type CurrencyExposure struct {
	Currency    string
	Assets      int64 // minor units, in Currency
	Liabilities int64
	Net         int64
	NetInBase   int64 // 0 when RateKnown is false
	RateKnown   bool
}

// NetWorthBreakdown is the net-worth report: base totals plus a per-currency
// breakdown. Currencies with no known rate are listed in MissingRates and left
// out of the base totals.
type NetWorthBreakdown struct {
	Base         string
	Assets       int64 // minor units, in Base
	Liabilities  int64
	Net          int64
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
			p.liabilities += ab.Balance
		} else {
			p.assets += ab.Balance
		}
	}

	out := NetWorthBreakdown{Base: base}

	for _, cur := range order {
		p := positions[cur]
		exposure := CurrencyExposure{
			Currency:    cur,
			Assets:      p.assets,
			Liabilities: p.liabilities,
			Net:         p.assets - p.liabilities,
		}

		rate, known := rateToBase(cur, base, rates)
		if known {
			exposure.RateKnown = true
			exposure.NetInBase = rate.Convert(exposure.Net)
			out.Assets += rate.Convert(p.assets)
			out.Liabilities += rate.Convert(p.liabilities)
		} else {
			out.MissingRates = append(out.MissingRates, cur)
		}

		out.ByCurrency = append(out.ByCurrency, exposure)
	}

	out.Net = out.Assets - out.Liabilities

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
