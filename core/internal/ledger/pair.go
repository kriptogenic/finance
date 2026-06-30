package ledger

import (
	"errors"
	"fmt"

	"finance/internal/entities"
)

// TransferFromLegs combines two ingested single legs — an expense (debit) and an
// income (credit) — into one transfer, when they are two sides of the same money
// move: equal amount and currency, touching two distinct accounts. The argument
// order does not matter.
//
// The result carries no category and keeps the debit leg's frozen rate/base
// amount, so a paired transfer leaves net worth unchanged (§5). It returns an
// error when the legs do not form a valid transfer, so a caller never merges a
// false pair. The returned transaction has no ID/ExternalID — the repository
// keeps the surviving row's identity.
func TransferFromLegs(a, b entities.Transaction) (entities.Transaction, error) {
	debit, credit := a, b
	if debit.Type == entities.TxIncome {
		debit, credit = credit, debit
	}

	switch {
	case debit.Type != entities.TxExpense || credit.Type != entities.TxIncome:
		return entities.Transaction{}, errors.New("a transfer pair needs one expense and one income leg")
	case debit.FromAccountID == nil || credit.ToAccountID == nil:
		return entities.Transaction{}, errors.New("transfer legs are missing their accounts")
	case *debit.FromAccountID == *credit.ToAccountID:
		return entities.Transaction{}, errors.New("transfer legs must touch two distinct accounts")
	case debit.Amount.Code() != credit.Amount.Code() || debit.Amount.Minor() != credit.Amount.Minor():
		return entities.Transaction{}, errors.New("transfer legs must share amount and currency")
	}

	date := debit.Date
	if credit.Date.Before(date) {
		date = credit.Date
	}

	return entities.Transaction{
		Date:            date,
		Type:            entities.TxTransfer,
		FromAccountID:   debit.FromAccountID,
		ToAccountID:     credit.ToAccountID,
		Amount:          debit.Amount,
		RateToBase:      debit.RateToBase,
		BaseAmount:      debit.BaseAmount,
		Tags:            unionTags(debit.Tags, credit.Tags),
		TransferGroupID: ptrString(pairGroupID(debit, credit)),
	}, nil
}

// pairGroupID is a stable, order-independent group key derived from both legs'
// external ids, so the same pair always yields the same transfer_group_id.
func pairGroupID(debit, credit entities.Transaction) string {
	lo, hi := legID(debit), legID(credit)
	if lo > hi {
		lo, hi = hi, lo
	}

	return fmt.Sprintf("tf:%s|%s", lo, hi)
}

func legID(t entities.Transaction) string {
	if t.ExternalID == nil {
		return ""
	}

	return *t.ExternalID
}

func unionTags(a, b []string) []string {
	seen := make(map[string]struct{}, len(a)+len(b))
	out := []string{}
	for _, group := range [][]string{a, b} {
		for _, tag := range group {
			if _, ok := seen[tag]; ok {
				continue
			}
			seen[tag] = struct{}{}
			out = append(out, tag)
		}
	}

	return out
}

func ptrString(s string) *string { return &s }
