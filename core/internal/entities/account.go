package entities

import (
	"time"

	"github.com/google/uuid"

	"finance/pkg/money"
)

// AccountKind is the asset/liability split that drives net worth (REQUIREMENTS §2).
type AccountKind string

const (
	KindAsset     AccountKind = "asset"
	KindLiability AccountKind = "liability"
)

// AccountType is the concrete account flavour. The asset/liability split is
// fixed by the type (enforced in the DB by accounts_kind_type_chk).
type AccountType string

const (
	TypeCash       AccountType = "cash"
	TypeDebitCard  AccountType = "debit_card"
	TypeDeposit    AccountType = "deposit"
	TypeCreditCard AccountType = "credit_card"
	TypeLoan       AccountType = "loan"
	// TypeReceivable is a per-person account holding what a friend owes you
	// after a split. Auto-archived once its balance returns to zero.
	TypeReceivable AccountType = "receivable"
)

// Account is a money bucket you own (asset) or owe (liability). It holds exactly
// one currency; balances are derived, never stored mutably (§5).
type Account struct {
	ID             uuid.UUID   `db:"id"`
	Name           string      `db:"name"`
	Kind           AccountKind `db:"kind"`
	Type           AccountType `db:"type"`
	Currency       string      `db:"currency"`        // ISO 4217 — the account's single currency
	OpeningBalance money.Money `db:"opening_balance"` // in Currency; for liabilities, positive = owed at start
	Archived       bool        `db:"archived"`
	CreatedAt      time.Time   `db:"created_at"`

	// Stored/exposed as include_in_net_worth; inverted here so the zero value
	// (false) means included — the default. Excluded accounts still record
	// transactions but drop out of net-worth totals and currency exposure. Reads
	// select NOT include_in_net_worth AS excluded_from_net_worth.
	ExcludedFromNetWorth bool `db:"excluded_from_net_worth"`

	CardLast4 *string `db:"card_last4"` // identifies the account for external ingest (e.g. card alerts)

	// deposit subtype
	InterestRate   *float64   `db:"interest_rate"`
	TermMonths     *int       `db:"term_months"`
	MaturityDate   *time.Time `db:"maturity_date"`
	Capitalization *bool      `db:"capitalization"`

	// credit_card subtype
	CreditLimit *money.Money `db:"credit_limit"` // in Currency

	// loan subtype
	Principal  *money.Money `db:"principal"` // in Currency
	StartDate  *time.Time   `db:"start_date"`
	PaymentDay *int         `db:"payment_day"`
}

func (a Account) IsAsset() bool     { return a.Kind == KindAsset }
func (a Account) IsLiability() bool { return a.Kind == KindLiability }

// ValidKindType reports whether the kind matches the type per the asset/
// liability split (§2). The DB enforces this too (accounts_kind_type_chk); we
// check it up front to return 400 rather than a 500 from a constraint error.
func (a Account) ValidKindType() bool {
	switch a.Type {
	case TypeCash, TypeDebitCard, TypeDeposit, TypeReceivable:
		return a.Kind == KindAsset
	case TypeCreditCard, TypeLoan:
		return a.Kind == KindLiability
	default:
		return false
	}
}
