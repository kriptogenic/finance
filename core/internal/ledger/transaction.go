package ledger

import (
	"errors"
	"fmt"
	"time"

	"finance/internal/entities"
	"finance/pkg/fx"
	"finance/pkg/money"
)

// NewTransaction is the validated input to BuildTransaction. The caller resolves
// the referenced accounts/category and passes the loaded entities so the engine
// can stay a pure function (no DB) and be unit-tested in isolation.
type NewTransaction struct {
	Date       time.Time
	Type       entities.TransactionType
	From       *entities.Account
	To         *entities.Account
	Category   *entities.Category
	Amount     money.Money
	ToAmount   *money.Money // cross-currency transfers only
	RateToBase *fx.Rate
	Note       *string
	Tags       []string
}

// BuildTransaction validates the per-type invariants (§4/§5), derives the
// transaction currency from the relevant account, resolves the transfer second
// leg, and freezes rate_to_base / base_amount at transaction time (§3). The
// returned Transaction has no ID/CreatedAt yet — the repository sets those.
//
// Every returned error is a client-input error (map to HTTP 400).
func BuildTransaction(in NewTransaction, base string) (entities.Transaction, error) {
	if in.Amount.Minor() <= 0 {
		return entities.Transaction{}, errors.New("amount must be positive")
	}

	if err := validateShape(in); err != nil {
		return entities.Transaction{}, err
	}

	tx := entities.Transaction{
		Date: in.Date,
		Type: in.Type,
		Note: in.Note,
		Tags: in.Tags,
	}
	if tx.Tags == nil {
		tx.Tags = []string{}
	}

	currency := assignBuckets(&tx, in)
	tx.Amount = money.New(in.Amount.Minor(), currency)

	if err := resolveSecondLeg(&tx, in); err != nil {
		return entities.Transaction{}, err
	}

	if err := freezeRate(&tx, in, currency, base); err != nil {
		return entities.Transaction{}, err
	}

	return tx, nil
}

func validateShape(in NewTransaction) error {
	switch in.Type {
	case entities.TxExpense:
		switch {
		case in.From == nil:
			return errors.New("expense requires a from account")
		case in.To != nil:
			return errors.New("expense must not have a to account")
		case in.Category == nil:
			return errors.New("expense requires a category")
		case in.Category.Type != entities.CategoryExpense:
			return errors.New("expense requires an expense category")
		}
	case entities.TxIncome:
		switch {
		case in.To == nil:
			return errors.New("income requires a to account")
		case in.From != nil:
			return errors.New("income must not have a from account")
		case in.Category == nil:
			return errors.New("income requires a category")
		case in.Category.Type != entities.CategoryIncome:
			return errors.New("income requires an income category")
		}
	case entities.TxTransfer:
		switch {
		case in.From == nil || in.To == nil:
			return errors.New("transfer requires from and to accounts")
		case in.Category != nil:
			return errors.New("transfer must not have a category")
		case in.From.ID == in.To.ID:
			return errors.New("transfer requires two distinct accounts")
		}
	default:
		return fmt.Errorf("unknown transaction type %q", in.Type)
	}

	return validateNotArchived(in)
}

func validateNotArchived(in NewTransaction) error {
	switch {
	case in.From != nil && in.From.Archived:
		return errors.New("from account is archived")
	case in.To != nil && in.To.Archived:
		return errors.New("to account is archived")
	case in.Category != nil && in.Category.Archived:
		return errors.New("category is archived")
	}

	return nil
}

// assignBuckets sets the account/category ids per type and returns the primary
// (amount) currency: the from account for expense/transfer, the to account for income.
func assignBuckets(tx *entities.Transaction, in NewTransaction) string {
	switch in.Type {
	case entities.TxExpense:
		tx.FromAccountID = &in.From.ID
		tx.CategoryID = &in.Category.ID

		return in.From.Currency
	case entities.TxIncome:
		tx.ToAccountID = &in.To.ID
		tx.CategoryID = &in.Category.ID

		return in.To.Currency
	default: // transfer
		tx.FromAccountID = &in.From.ID
		tx.ToAccountID = &in.To.ID

		return in.From.Currency
	}
}

func resolveSecondLeg(tx *entities.Transaction, in NewTransaction) error {
	if in.Type != entities.TxTransfer {
		if in.ToAmount != nil {
			return errors.New("to_amount is only valid for transfers")
		}

		return nil
	}

	if in.From.Currency == in.To.Currency {
		if in.ToAmount != nil && in.ToAmount.Minor() != in.Amount.Minor() {
			return errors.New("same-currency transfer must not change the amount")
		}

		return nil
	}

	// cross-currency: the credited leg is independent and required
	if in.ToAmount == nil || in.ToAmount.Minor() <= 0 {
		return errors.New("cross-currency transfer requires a positive to_amount")
	}

	toLeg := money.New(in.ToAmount.Minor(), in.To.Currency)
	tx.ToAmount = &toLeg

	return nil
}

// freezeRate stores rate_to_base and base_amount when the currency differs from
// base; when it equals base both stay nil (base_amount == amount implicitly).
func freezeRate(tx *entities.Transaction, in NewTransaction, currency, base string) error {
	if currency == base {
		return nil
	}

	if in.RateToBase == nil || !in.RateToBase.Valid() {
		return fmt.Errorf("rate_to_base is required for currency %s (base %s)", currency, base)
	}

	rate := *in.RateToBase
	baseAmount := FreezeBase(tx.Amount, rate, base)
	tx.RateToBase = &rate
	tx.BaseAmount = &baseAmount

	return nil
}
