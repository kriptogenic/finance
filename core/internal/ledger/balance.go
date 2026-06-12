// Package ledger holds the money core: deriving account balances from
// transactions and aggregating them into net worth. Everything here is a pure
// function of its inputs so it can be unit-tested without a database, as the
// project workflow requires.
package ledger

import (
	"fmt"

	money "github.com/Rhymond/go-money"

	"finance/internal/entities"
	"finance/pkg/fx"
)

// DeriveBalance computes an account's balance in its own currency (minor units)
// from its opening balance and the transactions touching it (§5):
//
//	asset.balance     = opening + inflows − outflows   (what you own)
//	liability.balance = opening − inflows + outflows   (what you owe)
//
// For a liability, an outflow (card spend, loan draw) increases what you owe and
// an inflow (repayment) reduces it — the mirror of an asset.
func DeriveBalance(acc entities.Account, txns []entities.Transaction) int64 {
	var inflow, outflow int64

	for _, t := range txns {
		if t.ToAccountID != nil && *t.ToAccountID == acc.ID {
			inflow += t.CreditAmount()
		}

		if t.FromAccountID != nil && *t.FromAccountID == acc.ID {
			outflow += t.Amount
		}
	}

	if acc.IsLiability() {
		return acc.OpeningBalance - inflow + outflow
	}

	return acc.OpeningBalance + inflow - outflow
}

// FreezeBase computes the base_amount to persist on a transaction, freezing the
// FX rate at transaction time so historical reports never re-convert (§3).
func FreezeBase(amount int64, rate fx.Rate) int64 {
	return rate.Convert(amount)
}

// AccountBalance pairs an account with its derived balance (in the account's
// currency), the input to net-worth aggregation.
type AccountBalance struct {
	Account entities.Account
	Balance int64
}

// NetWorth converts every balance to the base currency and returns
// Σ(assets) − Σ(liabilities). rates maps a currency code to its rate-to-base;
// the base currency itself needs no entry (it converts at identity).
func NetWorth(base string, balances []AccountBalance, rates map[string]fx.Rate) (*money.Money, error) {
	total := money.New(0, base)

	for _, ab := range balances {
		rate := fx.One()
		if ab.Account.Currency != base {
			r, ok := rates[ab.Account.Currency]
			if !ok || !r.Valid() {
				return nil, fmt.Errorf("ledger: missing rate for %s", ab.Account.Currency)
			}

			rate = r
		}

		leg := money.New(rate.Convert(ab.Balance), base)

		var err error
		if ab.Account.IsLiability() {
			total, err = total.Subtract(leg)
		} else {
			total, err = total.Add(leg)
		}

		if err != nil {
			return nil, fmt.Errorf("ledger: aggregate net worth: %w", err)
		}
	}

	return total, nil
}
