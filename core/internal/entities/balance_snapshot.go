package entities

import (
	"time"

	"finance/pkg/money"
)

// BalanceSnapshot is the latest balance an external source reported for a card.
// It is matched to an account via CardLast4 for reconciliation; the bank-derived
// balance is never used as the system of record, only compared against it.
type BalanceSnapshot struct {
	CardLast4  string      `db:"card_last4"`
	Bank       *string     `db:"bank"`
	Amount     money.Money `db:"amount"` // the reported balance, in its own currency
	Source     *string     `db:"source"`
	ReportedAt time.Time   `db:"reported_at"`
}
