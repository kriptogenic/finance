package ledger

import (
	"errors"
	"strings"
)

// SplitParticipant is one friend's share of a split bill. Amount is in the
// paying account's currency (minor units).
type SplitParticipant struct {
	Name   string
	Amount int64
}

// EvenSplit divides total across people, returning the floor per-person share
// and the leftover minor units. The remainder belongs to your own share so the
// parts always re-sum to total. people <= 0 yields a zero split.
func EvenSplit(total int64, people int) (perPerson, remainder int64) {
	if people <= 0 {
		return 0, total
	}

	perPerson = total / int64(people)
	remainder = total - perPerson*int64(people)

	return perPerson, remainder
}

// ValidateSplit checks a split breakdown. The bill total is derived as
// myShare + Σ participants, so re-splitting needs no stored original. Every
// returned error is a client-input error (map to HTTP 400).
func ValidateSplit(myShare int64, participants []SplitParticipant) error {
	// the expense leg keeps your share, and the DB requires a positive amount
	if myShare <= 0 {
		return errors.New("your share must be positive")
	}

	for _, p := range participants {
		if strings.TrimSpace(p.Name) == "" {
			return errors.New("each participant needs a name")
		}
		if p.Amount <= 0 {
			return errors.New("each participant's share must be positive")
		}
	}

	return nil
}
