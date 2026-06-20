package entities

import (
	"time"

	"github.com/google/uuid"
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
)

// Account is a money bucket you own (asset) or owe (liability). It holds exactly
// one currency; balances are derived, never stored mutably (§5).
type Account struct {
	ID             uuid.UUID
	Name           string
	Kind           AccountKind
	Type           AccountType
	Currency       string // ISO 4217
	OpeningBalance int64  // minor units; for liabilities, positive = owed at start
	Archived       bool
	CreatedAt      time.Time

	// Stored/exposed as include_in_net_worth; inverted here so the zero value
	// (false) means included — the default. Excluded accounts still record
	// transactions but drop out of net-worth totals and currency exposure.
	ExcludedFromNetWorth bool

	CardLast4 *string // identifies the account for external ingest (e.g. card alerts)

	// deposit subtype
	InterestRate   *float64
	TermMonths     *int
	MaturityDate   *time.Time
	Capitalization *bool

	// credit_card subtype
	CreditLimit *int64 // minor units

	// loan subtype
	Principal  *int64 // minor units
	StartDate  *time.Time
	PaymentDay *int
}

func (a Account) IsAsset() bool     { return a.Kind == KindAsset }
func (a Account) IsLiability() bool { return a.Kind == KindLiability }

// ValidKindType reports whether the kind matches the type per the asset/
// liability split (§2). The DB enforces this too (accounts_kind_type_chk); we
// check it up front to return 400 rather than a 500 from a constraint error.
func (a Account) ValidKindType() bool {
	switch a.Type {
	case TypeCash, TypeDebitCard, TypeDeposit:
		return a.Kind == KindAsset
	case TypeCreditCard, TypeLoan:
		return a.Kind == KindLiability
	default:
		return false
	}
}
