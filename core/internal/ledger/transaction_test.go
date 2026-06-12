package ledger_test

import (
	"github.com/google/uuid"

	"finance/internal/entities"
	"finance/internal/ledger"
	"finance/pkg/fx"
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
		Type: entities.TxExpense, From: &from, Category: &food, Amount: 1500,
	}, "USD")
	s.Require().NoError(err)

	s.Equal(entities.TxExpense, tx.Type)
	s.Equal(&from.ID, tx.FromAccountID)
	s.Nil(tx.ToAccountID)
	s.Equal(&food.ID, tx.CategoryID)
	s.Equal("USD", tx.Currency)
	s.Nil(tx.RateToBase, "no rate frozen when currency == base")
	s.Nil(tx.BaseAmount)
}

func (s *LedgerSuite) TestBuild_Expense_FreezesRate() {
	from := cashAcc("UZS")
	food := cat(entities.CategoryExpense)
	rate := fx.MustParseRate("0.000079") // UZS→USD-ish

	tx, err := ledger.BuildTransaction(ledger.NewTransaction{
		Type: entities.TxExpense, From: &from, Category: &food, Amount: 1_000_000, RateToBase: &rate,
	}, "USD")
	s.Require().NoError(err)

	s.Require().NotNil(tx.RateToBase)
	s.Require().NotNil(tx.BaseAmount)
	s.Equal(int64(79), *tx.BaseAmount, "1_000_000 × 0.000079 = 79")
}

func (s *LedgerSuite) TestBuild_Expense_MissingRateErrors() {
	from := cashAcc("UZS")
	food := cat(entities.CategoryExpense)

	_, err := ledger.BuildTransaction(ledger.NewTransaction{
		Type: entities.TxExpense, From: &from, Category: &food, Amount: 100,
	}, "USD")
	s.Require().Error(err)
	s.Contains(err.Error(), "rate_to_base")
}

func (s *LedgerSuite) TestBuild_Income() {
	to := cashAcc("USD")
	salary := cat(entities.CategoryIncome)

	tx, err := ledger.BuildTransaction(ledger.NewTransaction{
		Type: entities.TxIncome, To: &to, Category: &salary, Amount: 500000,
	}, "USD")
	s.Require().NoError(err)
	s.Equal(&to.ID, tx.ToAccountID)
	s.Nil(tx.FromAccountID)
	s.Equal("USD", tx.Currency)
}

func (s *LedgerSuite) TestBuild_Transfer_SameCurrency() {
	from := cashAcc("USD")
	to := cashAcc("USD")

	tx, err := ledger.BuildTransaction(ledger.NewTransaction{
		Type: entities.TxTransfer, From: &from, To: &to, Amount: 200_00,
	}, "USD")
	s.Require().NoError(err)
	s.Nil(tx.CategoryID, "transfers carry no category")
	s.Nil(tx.ToAmount, "same-currency transfer has no second leg")
	s.Nil(tx.ToCurrency)
}

func (s *LedgerSuite) TestBuild_Transfer_CrossCurrency() {
	from := cashAcc("USD")
	to := cashAcc("UZS")
	toAmount := int64(1_250_000_00)
	rate := fx.MustParseRate("1") // base is USD; from leg already in base

	tx, err := ledger.BuildTransaction(ledger.NewTransaction{
		Type: entities.TxTransfer, From: &from, To: &to, Amount: 100_00,
		ToAmount: &toAmount, RateToBase: &rate,
	}, "USD")
	s.Require().NoError(err)

	s.Equal("USD", tx.Currency)
	s.Require().NotNil(tx.ToAmount)
	s.Equal(int64(1_250_000_00), *tx.ToAmount)
	s.Require().NotNil(tx.ToCurrency)
	s.Equal("UZS", *tx.ToCurrency)
}

func (s *LedgerSuite) TestBuild_Transfer_CrossCurrency_NeedsToAmount() {
	from := cashAcc("USD")
	to := cashAcc("UZS")
	rate := fx.MustParseRate("1")

	_, err := ledger.BuildTransaction(ledger.NewTransaction{
		Type: entities.TxTransfer, From: &from, To: &to, Amount: 100_00, RateToBase: &rate,
	}, "USD")
	s.Require().Error(err)
	s.Contains(err.Error(), "to_amount")
}

func (s *LedgerSuite) TestBuild_RejectsShapeViolations() {
	from := cashAcc("USD")
	to := cashAcc("USD")
	expense := cat(entities.CategoryExpense)
	income := cat(entities.CategoryIncome)

	cases := map[string]ledger.NewTransaction{
		"expense without category": {Type: entities.TxExpense, From: &from, Amount: 1},
		"expense with income cat":  {Type: entities.TxExpense, From: &from, Category: &income, Amount: 1},
		"income without category":  {Type: entities.TxIncome, To: &to, Amount: 1},
		"transfer with category":   {Type: entities.TxTransfer, From: &from, To: &to, Category: &expense, Amount: 1},
		"self transfer":            {Type: entities.TxTransfer, From: &from, To: &from, Amount: 1},
		"non-positive amount":      {Type: entities.TxExpense, From: &from, Category: &expense, Amount: 0},
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
		Type: entities.TxExpense, From: &from, Category: &food, Amount: 1,
	}, "USD")
	s.Require().Error(err)
	s.Contains(err.Error(), "archived")
}
