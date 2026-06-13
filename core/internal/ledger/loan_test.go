package ledger_test

import (
	"math"
	"time"

	"finance/internal/ledger"
)

func (s *LedgerSuite) TestAmortize_FullyAmortizes() {
	const principal = int64(1_000_000)
	sched, err := ledger.Amortize(principal, 0.12, 12, time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC), 1)
	s.Require().NoError(err)
	s.Require().Len(sched.Rows, 12)

	// cross-check the annuity payment against the float formula (±1 minor unit)
	i := 0.12 / 12
	want := int64(math.Round(float64(principal) * i / (1 - math.Pow(1+i, -12))))
	s.InDelta(want, sched.MonthlyPayment, 1)

	var sumPrincipal, sumInterest, sumPayment int64
	prevInterest := int64(math.MaxInt64)
	for _, r := range sched.Rows {
		s.Equal(r.Principal+r.Interest, r.Payment, "row %d reconciles", r.Period)
		s.LessOrEqual(r.Interest, prevInterest, "interest is non-increasing")
		prevInterest = r.Interest
		sumPrincipal += r.Principal
		sumInterest += r.Interest
		sumPayment += r.Payment
	}

	s.Equal(int64(0), sched.Rows[len(sched.Rows)-1].Balance, "loan fully repaid")
	s.Equal(principal, sumPrincipal, "principal portions sum to the loan")
	s.Equal(sched.TotalInterest, sumInterest)
	s.Equal(sched.TotalPayment, sumPayment)
	s.Equal(principal+sched.TotalInterest, sched.TotalPayment)
}

func (s *LedgerSuite) TestAmortize_ZeroInterest() {
	sched, err := ledger.Amortize(1200, 0, 12, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), 1)
	s.Require().NoError(err)

	s.Equal(int64(0), sched.TotalInterest)
	s.Equal(int64(100), sched.MonthlyPayment)
	s.Equal(int64(0), sched.Rows[11].Balance)

	var sumPrincipal int64
	for _, r := range sched.Rows {
		s.Equal(int64(0), r.Interest)
		sumPrincipal += r.Principal
	}
	s.Equal(int64(1200), sumPrincipal)
}

func (s *LedgerSuite) TestAmortize_DueDates() {
	sched, err := ledger.Amortize(300000, 0.1, 3, time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC), 5)
	s.Require().NoError(err)
	s.Require().Len(sched.Rows, 3)
	s.Equal(time.Date(2026, 2, 5, 0, 0, 0, 0, time.UTC), sched.Rows[0].Date)
	s.Equal(time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC), sched.Rows[1].Date)
	s.Equal(time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC), sched.Rows[2].Date)
}

func (s *LedgerSuite) TestAmortize_Errors() {
	_, err := ledger.Amortize(0, 0.1, 12, time.Now(), 1)
	s.Require().Error(err)
	_, err = ledger.Amortize(1000, 0.1, 0, time.Now(), 1)
	s.Require().Error(err)
}
