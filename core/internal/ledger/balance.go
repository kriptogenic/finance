// Package ledger holds the money core: deriving account balances from
// transactions and aggregating them into net worth. Everything here is a pure
// function of its inputs so it can be unit-tested without a database, as the
// project workflow requires.
package ledger

import (
	"fmt"

	"finance/internal/entities"
	"finance/pkg/fx"
	"finance/pkg/money"
)

// DeriveBalance computes an account's balance in its own currency from its
// opening balance and the transactions touching it (§5):
//
//	asset.balance     = opening + inflows − outflows
//	liability.balance = opening − inflows + outflows
func DeriveBalance(acc entities.Account, txns []entities.Transaction) (money.Money, error) {
	inflow := money.Zero(acc.Currency)
	outflow := money.Zero(acc.Currency)

	var err error
	for _, t := range txns {
		if t.ToAccountID != nil && *t.ToAccountID == acc.ID {
			if inflow, err = inflow.Plus(t.CreditAmount()); err != nil {
				return money.Money{}, err
			}
		}

		if t.FromAccountID != nil && *t.FromAccountID == acc.ID {
			if outflow, err = outflow.Plus(t.Amount); err != nil {
				return money.Money{}, err
			}
		}
	}

	return Balance(acc.Kind, acc.OpeningBalance, inflow, outflow)
}

// Balance applies the derivation formula to pre-aggregated inflow/outflow totals.
// It is the single source of truth for the asset vs liability sign convention.
func Balance(kind entities.AccountKind, opening, inflow, outflow money.Money) (money.Money, error) {
	if kind == entities.KindLiability {
		net, err := opening.Minus(inflow)
		if err != nil {
			return money.Money{}, err
		}

		return net.Plus(outflow)
	}

	net, err := opening.Plus(inflow)
	if err != nil {
		return money.Money{}, err
	}

	return net.Minus(outflow)
}

// FreezeBase converts amount to base at the frozen rate, returning the
// base_amount to persist (§3).
func FreezeBase(amount money.Money, rate fx.Rate, base string) money.Money {
	return money.New(rate.Convert(amount.Minor()), base)
}

// AccountBalance pairs an account with its derived balance (in the account's
// currency), the input to net-worth aggregation.
type AccountBalance struct {
	Account entities.Account
	Balance money.Money
}

// NetWorth converts every balance to the base currency and returns
// Σ(assets) − Σ(liabilities). rates maps a currency code to its rate-to-base;
// the base currency itself needs no entry (it converts at identity).
func NetWorth(base string, balances []AccountBalance, rates map[string]fx.Rate) (money.Money, error) {
	total := money.Zero(base)

	for _, ab := range balances {
		rate := fx.One()
		if ab.Account.Currency != base {
			r, ok := rates[ab.Account.Currency]
			if !ok || !r.Valid() {
				return money.Money{}, fmt.Errorf("ledger: missing rate for %s", ab.Account.Currency)
			}

			rate = r
		}

		leg := money.New(rate.Convert(ab.Balance.Minor()), base)

		var err error
		if ab.Account.IsLiability() {
			total, err = total.Minus(leg)
		} else {
			total, err = total.Plus(leg)
		}

		if err != nil {
			return money.Money{}, fmt.Errorf("ledger: aggregate net worth: %w", err)
		}
	}

	return total, nil
}
