package entities

import (
	"time"

	"github.com/google/uuid"

	"finance/pkg/fx"
	"finance/pkg/money"
)

// TransactionType is the kind of money move. Per the bucket model (§2) every
// transaction moves value between two buckets (accounts and/or categories).
type TransactionType string

const (
	TxExpense  TransactionType = "expense"
	TxIncome   TransactionType = "income"
	TxTransfer TransactionType = "transfer"
)

// Transaction moves money between two buckets. Field applicability by type is
// enforced in the DB (transactions_shape_chk); see REQUIREMENTS §4.
type Transaction struct {
	ID            uuid.UUID
	Date          time.Time
	Type          TransactionType
	FromAccountID *uuid.UUID
	ToAccountID   *uuid.UUID
	CategoryID    *uuid.UUID

	Amount money.Money // primary leg, in its account's currency

	// transfers only: the credited leg, in the to_account's currency
	ToAmount *money.Money

	// frozen at transaction time; required when the currency != base (§3)
	RateToBase *fx.Rate
	BaseAmount *money.Money // derived: Amount × RateToBase, in base currency

	Note      *string
	Tags      []string
	CreatedAt time.Time

	// external ingest metadata (e.g. Telegram userbot); nil for UI transactions
	ExternalID      *string // stable idempotency key from the source
	TransferGroupID *string // ties paired transfer legs together

	// ties a split expense to its per-person receivable transfer legs
	SplitGroupID *uuid.UUID

	// the fiscal receipt linked to this transaction, if any (read-only; the FK
	// lives on receipts). Populated on read via a correlated subquery.
	ReceiptID *uuid.UUID
}

// CreditAmount is the value credited to the receiving account, in that
// account's currency. For a cross-currency transfer that is the second leg;
// otherwise the primary amount.
func (t Transaction) CreditAmount() money.Money {
	if t.Type == TxTransfer && t.ToAmount != nil {
		return *t.ToAmount
	}

	return t.Amount
}
