package ledger

import (
	"errors"
	"strings"

	"finance/pkg/money"
)

// SplitParticipant is one friend's share of a split bill. Amount is in the
// paying account's currency.
type SplitParticipant struct {
	Name   string
	Amount money.Money
}

// EvenSplit divides total across people, returning the floor per-person share
// and the leftover minor units. The remainder belongs to your own share so the
// parts always re-sum to total. people <= 0 yields a zero split.
func EvenSplit(total money.Money, people int) (perPerson, remainder money.Money) {
	code := total.Code()
	if people <= 0 {
		return money.Zero(code), total
	}

	per := total.Minor() / int64(people)
	rem := total.Minor() - per*int64(people)

	return money.New(per, code), money.New(rem, code)
}

// ValidateSplit checks a split breakdown. The bill total is derived as
// myShare + Σ participants, so re-splitting needs no stored original. Every
// returned error is a client-input error (map to HTTP 400).
func ValidateSplit(myShare money.Money, participants []SplitParticipant) error {
	// the expense leg keeps your share, and the DB requires a positive amount
	if myShare.Minor() <= 0 {
		return errors.New("your share must be positive")
	}

	for _, p := range participants {
		if strings.TrimSpace(p.Name) == "" {
			return errors.New("each participant needs a name")
		}
		if p.Amount.Minor() <= 0 {
			return errors.New("each participant's share must be positive")
		}
	}

	return nil
}
