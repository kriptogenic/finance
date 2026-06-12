package ledger_test

import (
	"github.com/google/uuid"

	"finance/internal/ledger"
	"finance/pkg/fx"
)

func (s *LedgerSuite) TestComputeNetWorth_Breakdown() {
	balances := []ledger.AccountBalance{
		{Account: asset(uuid.New(), "USD", 0), Balance: 1000_00},
		{Account: asset(uuid.New(), "EUR", 0), Balance: 500_00},
		{Account: liability(uuid.New(), "USD", 0), Balance: 200_00},
	}
	rates := map[string]fx.Rate{"EUR": fx.MustParseRate("1.10")}

	nw := ledger.ComputeNetWorth("USD", balances, rates)

	s.Equal("USD", nw.Base)
	s.Equal(int64(1000_00+550_00), nw.Assets) // EUR 500 @1.10 = 550
	s.Equal(int64(200_00), nw.Liabilities)
	s.Equal(int64(1350_00), nw.Net)
	s.Empty(nw.MissingRates)

	byCur := map[string]ledger.CurrencyExposure{}
	for _, e := range nw.ByCurrency {
		byCur[e.Currency] = e
	}
	s.Equal(int64(800_00), byCur["USD"].Net) // 1000 assets − 200 liabilities
	s.Equal(int64(800_00), byCur["USD"].NetInBase)
	s.Equal(int64(550_00), byCur["EUR"].NetInBase)
	s.True(byCur["EUR"].RateKnown)
}

func (s *LedgerSuite) TestComputeNetWorth_MissingRateExcludedFromTotal() {
	balances := []ledger.AccountBalance{
		{Account: asset(uuid.New(), "USD", 0), Balance: 1000_00},
		{Account: asset(uuid.New(), "JPY", 0), Balance: 90000},
	}

	nw := ledger.ComputeNetWorth("USD", balances, nil)

	s.Equal(int64(1000_00), nw.Assets, "JPY excluded from base total")
	s.Equal(int64(1000_00), nw.Net)
	s.Equal([]string{"JPY"}, nw.MissingRates)

	for _, e := range nw.ByCurrency {
		if e.Currency == "JPY" {
			s.False(e.RateKnown)
			s.Equal(int64(0), e.NetInBase)
			s.Equal(int64(90000), e.Net, "still reported in its own currency")
		}
	}
}
