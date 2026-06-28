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
	ID            uuid.UUID       `db:"id"`
	Date          time.Time       `db:"date"`
	Type          TransactionType `db:"type"`
	FromAccountID *uuid.UUID      `db:"from_account_id"`
	ToAccountID   *uuid.UUID      `db:"to_account_id"`
	CategoryID    *uuid.UUID      `db:"category_id"`

	Amount money.Money `db:"amount"` // primary leg, in its account's currency

	// transfers only: the credited leg, in the to_account's currency
	ToAmount *money.Money `db:"to_amount"`

	// frozen at transaction time; required when the currency != base (§3). Reads
	// select rate_to_base::text AS rate_to_base so fx.Rate scans from the decimal.
	RateToBase *fx.Rate     `db:"rate_to_base"`
	BaseAmount *money.Money `db:"base_amount"` // derived: Amount × RateToBase, in base currency

	Note      *string   `db:"note"`
	Tags      []string  `db:"tags"`
	CreatedAt time.Time `db:"created_at"`

	// external ingest metadata (e.g. Telegram userbot); nil for UI transactions
	ExternalID      *string `db:"external_id"`       // stable idempotency key from the source
	TransferGroupID *string `db:"transfer_group_id"` // ties paired transfer legs together

	// ties a split expense to its per-person receivable transfer legs
	SplitGroupID *uuid.UUID `db:"split_group_id"`

	// the fiscal receipt linked to this transaction, if any (read-only; the FK
	// lives on receipts). Populated on read via a correlated subquery.
	ReceiptID *uuid.UUID `db:"receipt_id"`
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
