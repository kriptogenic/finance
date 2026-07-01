package ledger_test

import (
	"time"

	"finance/internal/ledger"
	"finance/pkg/money"
)

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func (s *LedgerSuite) TestGenerateSchedule_FullyAmortizes() {
	const principal = int64(1_000_000)
	sched, err := ledger.GenerateSchedule(ledger.ScheduleParams{
		Principal:  money.New(principal, "UZS"),
		AnnualRate: 0.16,
		TermMonths: 12,
		Start:      date(2026, 1, 10),
		PaymentDay: 10,
	})
	s.Require().NoError(err)
	s.Require().Len(sched.Rows, 12)

	var sumPrincipal, sumInterest, sumPayment int64
	for _, r := range sched.Rows {
		s.Equal(r.Principal.Minor()+r.Interest.Minor(), r.Payment.Minor(), "row %d reconciles", r.Period)
		sumPrincipal += r.Principal.Minor()
		sumInterest += r.Interest.Minor()
		sumPayment += r.Payment.Minor()
	}

	s.Equal(int64(0), sched.Rows[11].Balance.Minor(), "loan fully repaid")
	s.Equal(principal, sumPrincipal, "principal portions sum to the loan")
	s.Equal(sched.TotalInterest.Minor(), sumInterest)
	s.Equal(principal+sumInterest, sumPayment)
}

// Every installment must land on a business day (no weekends, no holidays).
func (s *LedgerSuite) TestGenerateSchedule_RollsOffWeekendsAndHolidays() {
	holidays := ledger.NewHolidaySet([]time.Time{
		date(2026, 3, 21), // Navruz
		date(2026, 5, 9),  // Memorial Day
	})
	sched, err := ledger.GenerateSchedule(ledger.ScheduleParams{
		Principal:  money.New(5_000_000, "UZS"),
		AnnualRate: 0.16,
		TermMonths: 12,
		Start:      date(2026, 1, 10),
		PaymentDay: 10,
		Calendar:   holidays,
	})
	s.Require().NoError(err)

	for _, r := range sched.Rows {
		s.NotEqual(time.Saturday, r.Date.Weekday(), "period %d not on Saturday", r.Period)
		s.NotEqual(time.Sunday, r.Date.Weekday(), "period %d not on Sunday", r.Period)
		s.False(holidays.IsHoliday(r.Date), "period %d not on a holiday", r.Period)
	}
}

// A manual override sets a row's date verbatim, bypassing the calendar.
func (s *LedgerSuite) TestGenerateSchedule_OverrideWins() {
	forced := date(2026, 2, 28) // a Saturday, deliberately
	sched, err := ledger.GenerateSchedule(ledger.ScheduleParams{
		Principal:  money.New(1_000_000, "UZS"),
		AnnualRate: 0.16,
		TermMonths: 6,
		Start:      date(2026, 1, 10),
		PaymentDay: 10,
		Overrides:  map[int]time.Time{2: forced},
	})
	s.Require().NoError(err)
	s.Equal(forced, sched.Rows[1].Date)
}

// A later due date accrues more actual/365 interest for the same balance.
func (s *LedgerSuite) TestGenerateSchedule_LongerGapCostsMoreInterest() {
	base := ledger.ScheduleParams{
		Principal:  money.New(1_000_000, "UZS"),
		AnnualRate: 0.16,
		TermMonths: 6,
		Start:      date(2026, 1, 10),
		PaymentDay: 10,
	}
	early, err := ledger.GenerateSchedule(base)
	s.Require().NoError(err)

	shifted := base
	shifted.Overrides = map[int]time.Time{1: early.Rows[0].Date.AddDate(0, 0, 3)}
	late, err := ledger.GenerateSchedule(shifted)
	s.Require().NoError(err)

	s.Greater(late.Rows[0].Interest.Minor(), early.Rows[0].Interest.Minor())
}

func (s *LedgerSuite) TestGenerateSchedule_ZeroInterest() {
	sched, err := ledger.GenerateSchedule(ledger.ScheduleParams{
		Principal:  money.New(1200, "UZS"),
		AnnualRate: 0,
		TermMonths: 12,
		Start:      date(2026, 1, 1),
		PaymentDay: 1,
	})
	s.Require().NoError(err)

	var sumPrincipal int64
	for _, r := range sched.Rows {
		s.Equal(int64(0), r.Interest.Minor())
		sumPrincipal += r.Principal.Minor()
	}
	s.Equal(int64(1200), sumPrincipal)
	s.Equal(int64(0), sched.Rows[11].Balance.Minor())
}

func (s *LedgerSuite) TestGenerateSchedule_Errors() {
	_, err := ledger.GenerateSchedule(ledger.ScheduleParams{Principal: money.New(0, "UZS"), AnnualRate: 0.1, TermMonths: 12, Start: date(2026, 1, 1)})
	s.Require().Error(err)
	_, err = ledger.GenerateSchedule(ledger.ScheduleParams{Principal: money.New(1000, "UZS"), AnnualRate: 0.1, TermMonths: 0, Start: date(2026, 1, 1)})
	s.Require().Error(err)
}
