package ledger_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"finance/internal/entities"
	"finance/internal/ledger"
	"finance/pkg/fx"
	"finance/pkg/money"
)

type LedgerSuite struct {
	suite.Suite
}

func TestLedgerSuite(t *testing.T) {
	suite.Run(t, new(LedgerSuite))
}

func ptr[T any](v T) *T { return &v }

func asset(id uuid.UUID, currency string, opening int64) entities.Account {
	return entities.Account{ID: id, Kind: entities.KindAsset, Type: entities.TypeCash, Currency: currency, OpeningBalance: money.New(opening, currency)}
}

func liability(id uuid.UUID, currency string, opening int64) entities.Account {
	return entities.Account{ID: id, Kind: entities.KindLiability, Type: entities.TypeCreditCard, Currency: currency, OpeningBalance: money.New(opening, currency)}
}

func (s *LedgerSuite) derive(acc entities.Account, txns []entities.Transaction) int64 {
	b, err := ledger.DeriveBalance(acc, txns)
	s.Require().NoError(err)

	return b.Minor()
}

// --- balance derivation -----------------------------------------------------

func (s *LedgerSuite) TestDeriveBalance_Asset() {
	cash := asset(uuid.New(), "USD", 1000_00)
	cat := uuid.New()
	other := uuid.New()

	txns := []entities.Transaction{
		{Type: entities.TxIncome, ToAccountID: &cash.ID, CategoryID: &cat, Amount: money.New(500_00, "USD")},
		{Type: entities.TxExpense, FromAccountID: &cash.ID, CategoryID: &cat, Amount: money.New(300_00, "USD")},
		{Type: entities.TxTransfer, FromAccountID: &cash.ID, ToAccountID: &other, Amount: money.New(200_00, "USD")},
		{Type: entities.TxTransfer, FromAccountID: &other, ToAccountID: &cash.ID, Amount: money.New(100_00, "USD")},
	}

	// 1000 + 500 − 300 − 200 + 100 = 1100
	s.Equal(int64(1100_00), s.derive(cash, txns))
}

func (s *LedgerSuite) TestDeriveBalance_Liability() {
	card := liability(uuid.New(), "USD", 0)
	bank := uuid.New()
	cat := uuid.New()

	txns := []entities.Transaction{
		// spend on the card → owe more
		{Type: entities.TxExpense, FromAccountID: &card.ID, CategoryID: &cat, Amount: money.New(300_00, "USD")},
		// pay the bill from a bank account → owe less
		{Type: entities.TxTransfer, FromAccountID: &bank, ToAccountID: &card.ID, Amount: money.New(100_00, "USD")},
	}

	// owed = 0 − 100 (repaid) + 300 (spent) = 200
	s.Equal(int64(200_00), s.derive(card, txns))
}

func (s *LedgerSuite) TestDeriveBalance_CrossCurrencyTransfer() {
	usd := asset(uuid.New(), "USD", 1000_00)
	uzs := asset(uuid.New(), "UZS", 0)

	// 100.00 USD leaves usd, 1_250_000.00 UZS lands in uzs
	tx := entities.Transaction{
		Type:          entities.TxTransfer,
		FromAccountID: &usd.ID,
		ToAccountID:   &uzs.ID,
		Amount:        money.New(100_00, "USD"),
		ToAmount:      ptr(money.New(1_250_000_00, "UZS")),
	}
	txns := []entities.Transaction{tx}

	s.Equal(int64(900_00), s.derive(usd, txns), "from leg debits the primary amount")
	s.Equal(int64(1_250_000_00), s.derive(uzs, txns), "to leg credits the second amount")
}

// --- rate freezing ----------------------------------------------------------

func (s *LedgerSuite) TestFreezeBase_RoundsHalfAwayFromZero() {
	freeze := func(amount int64, rate string) int64 {
		return ledger.FreezeBase(money.New(amount, "USD"), fx.MustParseRate(rate), "UZS").Minor()
	}
	s.Equal(int64(101), freeze(100, "1.005"), "100×1.005 = 100.5 → 101")
	s.Equal(int64(100), freeze(100, "1.004"), "100×1.004 = 100.4 → 100")
	s.Equal(int64(-2), freeze(-3, "0.5"), "-3×0.5 = -1.5 → -2")
	s.Equal(int64(1_250_050), freeze(100, "12500.50"), "exact, no float drift")
}

// --- net worth across currencies --------------------------------------------

func (s *LedgerSuite) TestNetWorth_MultiCurrency() {
	balances := []ledger.AccountBalance{
		{Account: asset(uuid.New(), "USD", 0), Balance: money.New(1000_00, "USD")},    // 1000.00 USD
		{Account: asset(uuid.New(), "EUR", 0), Balance: money.New(500_00, "EUR")},     // 500.00 EUR @1.10 = 550.00
		{Account: liability(uuid.New(), "USD", 0), Balance: money.New(200_00, "USD")}, // owe 200.00 USD
	}
	rates := map[string]fx.Rate{"EUR": fx.MustParseRate("1.10")}

	nw, err := ledger.NetWorth("USD", balances, rates)
	s.Require().NoError(err)
	// 1000 + 550 − 200 = 1350
	s.Equal(int64(1350_00), nw.Amount())
	s.Equal("USD", nw.Currency().Code)
}

func (s *LedgerSuite) TestNetWorth_TransferDoesNotChangeIt() {
	a := asset(uuid.New(), "USD", 1000_00)
	b := asset(uuid.New(), "USD", 0)

	withoutTransfer := []ledger.AccountBalance{
		{Account: a, Balance: money.New(s.derive(a, nil), "USD")},
		{Account: b, Balance: money.New(s.derive(b, nil), "USD")},
	}
	before, err := ledger.NetWorth("USD", withoutTransfer, nil)
	s.Require().NoError(err)

	txns := []entities.Transaction{
		{Type: entities.TxTransfer, FromAccountID: &a.ID, ToAccountID: &b.ID, Amount: money.New(300_00, "USD")},
	}
	withTransfer := []ledger.AccountBalance{
		{Account: a, Balance: money.New(s.derive(a, txns), "USD")},
		{Account: b, Balance: money.New(s.derive(b, txns), "USD")},
	}
	after, err := ledger.NetWorth("USD", withTransfer, nil)
	s.Require().NoError(err)

	s.Equal(before.Amount(), after.Amount(), "a transfer only moves value between accounts")
	s.Equal(int64(1000_00), after.Amount())
}

func (s *LedgerSuite) TestNetWorth_MissingRateErrors() {
	balances := []ledger.AccountBalance{
		{Account: asset(uuid.New(), "EUR", 0), Balance: money.New(100_00, "EUR")},
	}

	_, err := ledger.NetWorth("USD", balances, nil)
	s.Require().Error(err)
	s.Contains(err.Error(), "EUR")
}
