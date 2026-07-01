package ledger

import (
	"errors"
	"math/big"
	"time"

	"finance/pkg/money"
)

// Calendar reports non-working days that push a payment to the next business
// day. Weekends are always non-working; a Calendar adds public holidays.
type Calendar interface {
	IsHoliday(day time.Time) bool
}

// HolidaySet is a Calendar backed by an explicit set of holiday dates. Dates are
// compared at day granularity in UTC.
type HolidaySet map[string]struct{}

// NewHolidaySet builds a HolidaySet from a list of holiday dates.
func NewHolidaySet(days []time.Time) HolidaySet {
	set := make(HolidaySet, len(days))
	for _, d := range days {
		set[dayKey(d)] = struct{}{}
	}

	return set
}

func (h HolidaySet) IsHoliday(day time.Time) bool {
	_, ok := h[dayKey(day)]

	return ok
}

func dayKey(t time.Time) string { return t.UTC().Format("2006-01-02") }

// ScheduleParams describes a fixed-rate annuity loan whose real payment dates
// slide off weekends/holidays. annualRate is a fraction (0.16 == 16%).
type ScheduleParams struct {
	Principal  money.Money
	AnnualRate float64
	TermMonths int
	Start      time.Time
	PaymentDay int
	Calendar   Calendar          // nil => weekends only
	Overrides  map[int]time.Time // period => final date, wins over the calendar
}

// GenerateSchedule builds the stored repayment plan. Each installment falls on
// the payment day rolled forward to the next business day (or a manual
// override); interest accrues actual/365 on the real day gap, and the annuity
// payment is re-solved each period over the remaining balance and term so the
// loan still clears at the final installment ("recompute forward").
func GenerateSchedule(p ScheduleParams) (AmortizationSchedule, error) {
	code := p.Principal.Code()
	principalMinor := p.Principal.Minor()
	if principalMinor <= 0 {
		return AmortizationSchedule{}, errors.New("loan principal must be positive")
	}
	if p.TermMonths <= 0 {
		return AmortizationSchedule{}, errors.New("loan term must be positive")
	}

	monthlyRate := new(big.Rat).Quo(new(big.Rat).SetFloat64(p.AnnualRate), big.NewRat(12, 1))
	dailyRate := new(big.Rat).Quo(new(big.Rat).SetFloat64(p.AnnualRate), big.NewRat(365, 1))

	schedule := AmortizationSchedule{Rows: make([]AmortizationRow, 0, p.TermMonths)}

	var totalPayment, totalInterest int64
	balance := principalMinor
	prevDate := p.Start
	for period := 1; period <= p.TermMonths; period++ {
		date := p.Overrides[period]
		if date.IsZero() {
			date = rollForward(dueDate(p.Start, period, p.PaymentDay), p.Calendar)
		}
		days := int64(date.Sub(prevDate) / (24 * time.Hour))

		interest := accrue(balance, dailyRate, days)
		remaining := p.TermMonths - period + 1
		payment := monthlyPayment(balance, monthlyRate, remaining)

		principalPaid := payment - interest
		pay := payment
		// the final installment (or any that would overshoot) clears the balance
		if period == p.TermMonths || principalPaid >= balance {
			principalPaid = balance
			pay = balance + interest
		}
		balance -= principalPaid

		schedule.Rows = append(schedule.Rows, AmortizationRow{
			Period:    period,
			Date:      date,
			Payment:   money.New(pay, code),
			Principal: money.New(principalPaid, code),
			Interest:  money.New(interest, code),
			Balance:   money.New(balance, code),
		})
		totalPayment += pay
		totalInterest += interest
		prevDate = date

		if balance <= 0 {
			break
		}
	}

	schedule.MonthlyPayment = schedule.Rows[0].Payment
	schedule.TotalPayment = money.New(totalPayment, code)
	schedule.TotalInterest = money.New(totalInterest, code)

	return schedule, nil
}

// accrue is actual/365 interest for a day gap: round(balance × dailyRate × days).
func accrue(balance int64, dailyRate *big.Rat, days int64) int64 {
	x := new(big.Rat).SetInt64(balance)
	x.Mul(x, new(big.Rat).SetInt64(days))
	x.Mul(x, dailyRate)

	return roundRat(x)
}

// rollForward advances t past weekends and calendar holidays to a business day.
func rollForward(t time.Time, cal Calendar) time.Time {
	for isNonWorking(t, cal) {
		t = t.AddDate(0, 0, 1)
	}

	return t
}

func isNonWorking(t time.Time, cal Calendar) bool {
	switch t.Weekday() {
	case time.Saturday, time.Sunday:
		return true
	}

	return cal != nil && cal.IsHoliday(t)
}
