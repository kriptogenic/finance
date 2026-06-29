package ledger

import (
	"github.com/google/uuid"

	"finance/internal/entities"
	"finance/pkg/money"
)

// ReconRow pairs an account with the latest balance an external source reported
// for its card. Delta is meaningful only when CurrencyMatch is true.
type ReconRow struct {
	Account  entities.Account
	Snapshot entities.BalanceSnapshot
	// Derived is the figure comparable to the bank's reported balance, in the
	// account's currency. For a credit card the bank reports available credit,
	// so this is credit_limit − owed; for other accounts it's the plain balance.
	Derived       money.Money
	Delta         money.Money // reported − derived (valid when CurrencyMatch)
	CurrencyMatch bool
	InSync        bool
}

// Reconcile matches reported card balances to accounts by card_last4 and compares
// each against the account's derived balance. Only matched pairs are returned, in
// account order. Pure: derived balances are supplied pre-computed (e.g. from SQL).
func Reconcile(snaps []entities.BalanceSnapshot, accounts []entities.Account, balances map[uuid.UUID]money.Money) []ReconRow {
	byCard := make(map[string]entities.BalanceSnapshot, len(snaps))
	for _, s := range snaps {
		byCard[s.CardLast4] = s
	}

	var rows []ReconRow
	for _, acc := range accounts {
		if acc.CardLast4 == nil {
			continue
		}
		snap, ok := byCard[*acc.CardLast4]
		if !ok {
			continue
		}

		derived := balances[acc.ID]
		// A credit card's bank balance is available credit (limit − owed), so
		// convert the derived owed amount to match before comparing.
		if acc.Type == entities.TypeCreditCard && acc.CreditLimit != nil {
			if avail, err := acc.CreditLimit.Minus(derived); err == nil {
				derived = avail
			}
		}
		match := snap.Amount.Code() == acc.Currency
		row := ReconRow{
			Account:       acc,
			Snapshot:      snap,
			Derived:       derived,
			CurrencyMatch: match,
		}
		if match {
			delta, err := snap.Amount.Minus(derived)
			if err == nil {
				row.Delta = delta
				row.InSync = delta.IsZero()
			}
		}

		rows = append(rows, row)
	}

	return rows
}
