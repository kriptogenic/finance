package ledger

import (
	"errors"
	"math/big"
	"time"

	"finance/pkg/money"
)

// AmortizationRow is one scheduled installment of a fully-amortizing loan.
type AmortizationRow struct {
	Period    int
	Date      time.Time
	Payment   money.Money // = Principal + Interest
	Principal money.Money
	Interest  money.Money
	Balance   money.Money // remaining principal after this installment
}

// AmortizationSchedule is the full repayment plan plus its totals.
type AmortizationSchedule struct {
	MonthlyPayment money.Money
	TotalPayment   money.Money
	TotalInterest  money.Money
	Rows           []AmortizationRow
}

// Amortize builds the schedule for a fixed-rate, fully-amortizing loan using the
// standard annuity formula. Math runs on integer minor units (re-wrapped in the
// loan currency); interest each period is round(balance × monthlyRate) so the
// rows reconcile exactly. annualRate is a fraction (0.16 == 16%).
func Amortize(principal money.Money, annualRate float64, termMonths int, start time.Time, paymentDay int) (AmortizationSchedule, error) {
	code := principal.Code()
	principalMinor := principal.Minor()
	if principalMinor <= 0 {
		return AmortizationSchedule{}, errors.New("loan principal must be positive")
	}
	if termMonths <= 0 {
		return AmortizationSchedule{}, errors.New("loan term must be positive")
	}

	monthlyRate := new(big.Rat).Quo(new(big.Rat).SetFloat64(annualRate), big.NewRat(12, 1))
	payment := monthlyPayment(principalMinor, monthlyRate, termMonths)

	schedule := AmortizationSchedule{
		MonthlyPayment: money.New(payment, code),
		Rows:           make([]AmortizationRow, 0, termMonths),
	}

	var totalPayment, totalInterest int64
	balance := principalMinor
	for period := 1; period <= termMonths; period++ {
		interest := roundRat(new(big.Rat).Mul(new(big.Rat).SetInt64(balance), monthlyRate))

		principalPaid := payment - interest
		pay := payment
		// the last installment (or any that would overshoot) clears the balance
		if period == termMonths || principalPaid >= balance {
			principalPaid = balance
			pay = balance + interest
		}
		balance -= principalPaid

		schedule.Rows = append(schedule.Rows, AmortizationRow{
			Period:    period,
			Date:      dueDate(start, period, paymentDay),
			Payment:   money.New(pay, code),
			Principal: money.New(principalPaid, code),
			Interest:  money.New(interest, code),
			Balance:   money.New(balance, code),
		})
		totalPayment += pay
		totalInterest += interest

		if balance <= 0 {
			break
		}
	}

	schedule.TotalPayment = money.New(totalPayment, code)
	schedule.TotalInterest = money.New(totalInterest, code)

	return schedule, nil
}

// monthlyPayment = P·i·(1+i)^n / ((1+i)^n − 1), or P/n when the rate is zero.
func monthlyPayment(principal int64, monthlyRate *big.Rat, termMonths int) int64 {
	if monthlyRate.Sign() == 0 {
		return roundRat(new(big.Rat).SetFrac64(principal, int64(termMonths)))
	}

	onePlusI := new(big.Rat).Add(big.NewRat(1, 1), monthlyRate)
	pow := ratPow(onePlusI, termMonths)

	num := new(big.Rat).Mul(new(big.Rat).SetInt64(principal), monthlyRate)
	num.Mul(num, pow)
	den := new(big.Rat).Sub(pow, big.NewRat(1, 1))

	return roundRat(new(big.Rat).Quo(num, den))
}

func ratPow(base *big.Rat, n int) *big.Rat {
	result := big.NewRat(1, 1)
	for k := 0; k < n; k++ {
		result.Mul(result, base)
	}

	return result
}

// dueDate is the installment date: paymentDay of the month, period months after
// the start date, clamped to the month length.
func dueDate(start time.Time, period, paymentDay int) time.Time {
	total := int(start.Month()) - 1 + period
	year := start.Year() + total/12
	month := time.Month(total%12 + 1)

	day := paymentDay
	if day <= 0 {
		day = start.Day()
	}
	lastDay := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
	if day > lastDay {
		day = lastDay
	}

	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func roundRat(x *big.Rat) int64 {
	quo := new(big.Int)
	rem := new(big.Int)
	quo.QuoRem(x.Num(), x.Denom(), rem)

	twiceRem := new(big.Int).Abs(rem)
	twiceRem.Lsh(twiceRem, 1)
	if twiceRem.Cmp(x.Denom()) >= 0 {
		if x.Sign() < 0 {
			quo.Sub(quo, big.NewInt(1))
		} else {
			quo.Add(quo, big.NewInt(1))
		}
	}

	return quo.Int64()
}
