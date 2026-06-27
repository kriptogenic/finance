package ledger_test

import (
	"github.com/google/uuid"

	"finance/internal/ledger"
	"finance/pkg/fx"
	"finance/pkg/money"
)

func (s *LedgerSuite) TestComputeNetWorth_Breakdown() {
	balances := []ledger.AccountBalance{
		{Account: asset(uuid.New(), "USD", 0), Balance: money.New(1000_00, "USD")},
		{Account: asset(uuid.New(), "EUR", 0), Balance: money.New(500_00, "EUR")},
		{Account: liability(uuid.New(), "USD", 0), Balance: money.New(200_00, "USD")},
	}
	rates := map[string]fx.Rate{"EUR": fx.MustParseRate("1.10")}

	nw := ledger.ComputeNetWorth("USD", balances, rates)

	s.Equal("USD", nw.Base)
	s.Equal(int64(1000_00+550_00), nw.Assets.Minor()) // EUR 500 @1.10 = 550
	s.Equal(int64(200_00), nw.Liabilities.Minor())
	s.Equal(int64(1350_00), nw.Net.Minor())
	s.Empty(nw.MissingRates)

	byCur := map[string]ledger.CurrencyExposure{}
	for _, e := range nw.ByCurrency {
		byCur[e.Currency] = e
	}
	s.Equal(int64(800_00), byCur["USD"].Net.Minor()) // 1000 assets − 200 liabilities
	s.Equal(int64(800_00), byCur["USD"].NetInBase.Minor())
	s.Equal(int64(550_00), byCur["EUR"].NetInBase.Minor())
	s.True(byCur["EUR"].RateKnown)
}

func (s *LedgerSuite) TestComputeNetWorth_MissingRateExcludedFromTotal() {
	balances := []ledger.AccountBalance{
		{Account: asset(uuid.New(), "USD", 0), Balance: money.New(1000_00, "USD")},
		{Account: asset(uuid.New(), "JPY", 0), Balance: money.New(90000, "JPY")},
	}

	nw := ledger.ComputeNetWorth("USD", balances, nil)

	s.Equal(int64(1000_00), nw.Assets.Minor(), "JPY excluded from base total")
	s.Equal(int64(1000_00), nw.Net.Minor())
	s.Equal([]string{"JPY"}, nw.MissingRates)

	for _, e := range nw.ByCurrency {
		if e.Currency == "JPY" {
			s.False(e.RateKnown)
			s.Equal(int64(0), e.NetInBase.Minor())
			s.Equal(int64(90000), e.Net.Minor(), "still reported in its own currency")
		}
	}
}
