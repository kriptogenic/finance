package ledger_test

import (
	"github.com/google/uuid"

	"finance/internal/entities"
	"finance/internal/ledger"
	"finance/pkg/fx"
	"finance/pkg/money"
)

func cashAcc(currency string) entities.Account {
	return entities.Account{ID: uuid.New(), Kind: entities.KindAsset, Type: entities.TypeCash, Currency: currency}
}

func cat(t entities.CategoryType) entities.Category {
	return entities.Category{ID: uuid.New(), Type: t}
}

func (s *LedgerSuite) TestBuild_Expense_BaseCurrency() {
	from := cashAcc("USD")
	food := cat(entities.CategoryExpense)

	tx, err := ledger.BuildTransaction(ledger.NewTransaction{
		Type: entities.TxExpense, From: &from, Category: &food, Amount: money.New(1500, "USD"),
	}, "USD")
	s.Require().NoError(err)

	s.Equal(entities.TxExpense, tx.Type)
	s.Equal(&from.ID, tx.FromAccountID)
	s.Nil(tx.ToAccountID)
	s.Equal(&food.ID, tx.CategoryID)
	s.Equal("USD", tx.Amount.Code())
	s.Nil(tx.RateToBase, "no rate frozen when currency == base")
	s.Nil(tx.BaseAmount)
}

func (s *LedgerSuite) TestBuild_Expense_FreezesRate() {
	from := cashAcc("UZS")
	food := cat(entities.CategoryExpense)
	rate := fx.MustParseRate("0.000079") // UZS→USD-ish

	tx, err := ledger.BuildTransaction(ledger.NewTransaction{
		Type: entities.TxExpense, From: &from, Category: &food, Amount: money.New(1_000_000, "UZS"), RateToBase: &rate,
	}, "USD")
	s.Require().NoError(err)

	s.Require().NotNil(tx.RateToBase)
	s.Require().NotNil(tx.BaseAmount)
	s.Equal(int64(79), tx.BaseAmount.Minor(), "1_000_000 × 0.000079 = 79")
	s.Equal("USD", tx.BaseAmount.Code())
}

func (s *LedgerSuite) TestBuild_Expense_MissingRateErrors() {
	from := cashAcc("UZS")
	food := cat(entities.CategoryExpense)

	_, err := ledger.BuildTransaction(ledger.NewTransaction{
		Type: entities.TxExpense, From: &from, Category: &food, Amount: money.New(100, "UZS"),
	}, "USD")
	s.Require().Error(err)
	s.Contains(err.Error(), "rate_to_base")
}

func (s *LedgerSuite) TestBuild_Income() {
	to := cashAcc("USD")
	salary := cat(entities.CategoryIncome)

	tx, err := ledger.BuildTransaction(ledger.NewTransaction{
		Type: entities.TxIncome, To: &to, Category: &salary, Amount: money.New(500000, "USD"),
	}, "USD")
	s.Require().NoError(err)
	s.Equal(&to.ID, tx.ToAccountID)
	s.Nil(tx.FromAccountID)
	s.Equal("USD", tx.Amount.Code())
}

func (s *LedgerSuite) TestBuild_Transfer_SameCurrency() {
	from := cashAcc("USD")
	to := cashAcc("USD")

	tx, err := ledger.BuildTransaction(ledger.NewTransaction{
		Type: entities.TxTransfer, From: &from, To: &to, Amount: money.New(200_00, "USD"),
	}, "USD")
	s.Require().NoError(err)
	s.Nil(tx.CategoryID, "transfers carry no category")
	s.Nil(tx.ToAmount, "same-currency transfer has no second leg")
}

func (s *LedgerSuite) TestBuild_Transfer_CrossCurrency() {
	from := cashAcc("USD")
	to := cashAcc("UZS")
	toAmount := money.New(1_250_000_00, "UZS")
	rate := fx.MustParseRate("1") // base is USD; from leg already in base

	tx, err := ledger.BuildTransaction(ledger.NewTransaction{
		Type: entities.TxTransfer, From: &from, To: &to, Amount: money.New(100_00, "USD"),
		ToAmount: &toAmount, RateToBase: &rate,
	}, "USD")
	s.Require().NoError(err)

	s.Equal("USD", tx.Amount.Code())
	s.Require().NotNil(tx.ToAmount)
	s.Equal(int64(1_250_000_00), tx.ToAmount.Minor())
	s.Equal("UZS", tx.ToAmount.Code())
}

func (s *LedgerSuite) TestBuild_Transfer_CrossCurrency_NeedsToAmount() {
	from := cashAcc("USD")
	to := cashAcc("UZS")
	rate := fx.MustParseRate("1")

	_, err := ledger.BuildTransaction(ledger.NewTransaction{
		Type: entities.TxTransfer, From: &from, To: &to, Amount: money.New(100_00, "USD"), RateToBase: &rate,
	}, "USD")
	s.Require().Error(err)
	s.Contains(err.Error(), "to_amount")
}

func (s *LedgerSuite) TestBuild_RejectsShapeViolations() {
	from := cashAcc("USD")
	to := cashAcc("USD")
	expense := cat(entities.CategoryExpense)
	income := cat(entities.CategoryIncome)
	one := money.New(1, "USD")

	cases := map[string]ledger.NewTransaction{
		"expense without category": {Type: entities.TxExpense, From: &from, Amount: one},
		"expense with income cat":  {Type: entities.TxExpense, From: &from, Category: &income, Amount: one},
		"income without category":  {Type: entities.TxIncome, To: &to, Amount: one},
		"transfer with category":   {Type: entities.TxTransfer, From: &from, To: &to, Category: &expense, Amount: one},
		"self transfer":            {Type: entities.TxTransfer, From: &from, To: &from, Amount: one},
		"non-positive amount":      {Type: entities.TxExpense, From: &from, Category: &expense, Amount: money.New(0, "USD")},
	}

	for name, in := range cases {
		_, err := ledger.BuildTransaction(in, "USD")
		s.Require().Errorf(err, "expected error for %q", name)
	}
}

func (s *LedgerSuite) TestBuild_RejectsArchived() {
	from := cashAcc("USD")
	from.Archived = true
	food := cat(entities.CategoryExpense)

	_, err := ledger.BuildTransaction(ledger.NewTransaction{
		Type: entities.TxExpense, From: &from, Category: &food, Amount: money.New(1, "USD"),
	}, "USD")
	s.Require().Error(err)
	s.Contains(err.Error(), "archived")
}
