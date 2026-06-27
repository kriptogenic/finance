package entities

import (
	"time"

	"finance/pkg/money"
)

// BalanceSnapshot is the latest balance an external source reported for a card.
// It is matched to an account via CardLast4 for reconciliation; the bank-derived
// balance is never used as the system of record, only compared against it.
type BalanceSnapshot struct {
	CardLast4  string
	Bank       *string
	Amount     money.Money // the reported balance, in its own currency
	Source     *string
	ReportedAt time.Time
}
