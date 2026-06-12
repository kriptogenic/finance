// Package fx holds the foreign-exchange rate value type and the one piece of
// money math that isn't plain integer addition: converting a minor-unit amount
// to the base currency by a stored rate. Conversion is exact (math/big) and
// rounds half away from zero, so a frozen rate always reproduces the same
// base_amount (REQUIREMENTS §3/§4).
package fx

import (
	"fmt"
	"math/big"
)

// Rate is units of base currency per one unit of the quote currency, applied to
// minor-unit amounts: base_amount = round(amount × rate). The zero Rate is
// invalid; use ParseRate or One.
type Rate struct {
	r *big.Rat
}

// One is the identity rate, used when a transaction is already in the base currency.
func One() Rate {
	return Rate{r: big.NewRat(1, 1)}
}

// ParseRate parses a decimal string such as "12500.50" or a fraction "1/3".
func ParseRate(s string) (Rate, error) {
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		return Rate{}, fmt.Errorf("fx: invalid rate %q", s)
	}

	return Rate{r: r}, nil
}

// MustParseRate is ParseRate that panics on error; for tests and constants.
func MustParseRate(s string) Rate {
	r, err := ParseRate(s)
	if err != nil {
		panic(err)
	}

	return r
}

// Valid reports whether the rate was initialised.
func (rate Rate) Valid() bool { return rate.r != nil }

// String renders the rate as a decimal with up to 10 fractional digits,
// matching the NUMERIC(20,10) column.
func (rate Rate) String() string {
	if rate.r == nil {
		return ""
	}

	return rate.r.FloatString(10)
}

// Convert applies the rate to a minor-unit amount, rounding half away from zero.
func (rate Rate) Convert(amount int64) int64 {
	if rate.r == nil {
		return amount
	}

	prod := new(big.Rat).Mul(new(big.Rat).SetInt64(amount), rate.r)

	return roundRat(prod)
}

func roundRat(x *big.Rat) int64 {
	quo := new(big.Int)
	rem := new(big.Int)
	quo.QuoRem(x.Num(), x.Denom(), rem)

	// round half away from zero: 2·|rem| >= denom bumps the magnitude
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
