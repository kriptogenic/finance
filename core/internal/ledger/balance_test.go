package ledger_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"finance/internal/entities"
	"finance/internal/ledger"
	"finance/pkg/fx"
)

type LedgerSuite struct {
	suite.Suite
}

func TestLedgerSuite(t *testing.T) {
	suite.Run(t, new(LedgerSuite))
}

func ptr[T any](v T) *T { return &v }

func asset(id uuid.UUID, currency string, opening int64) entities.Account {
	return entities.Account{ID: id, Kind: entities.KindAsset, Type: entities.TypeCash, Currency: currency, OpeningBalance: opening}
}

func liability(id uuid.UUID, currency string, opening int64) entities.Account {
	return entities.Account{ID: id, Kind: entities.KindLiability, Type: entities.TypeCreditCard, Currency: currency, OpeningBalance: opening}
}

// --- balance derivation -----------------------------------------------------

func (s *LedgerSuite) TestDeriveBalance_Asset() {
	cash := asset(uuid.New(), "USD", 1000_00)
	cat := uuid.New()
	other := uuid.New()

	txns := []entities.Transaction{
		{Type: entities.TxIncome, ToAccountID: &cash.ID, CategoryID: &cat, Amount: 500_00, Currency: "USD"},
		{Type: entities.TxExpense, FromAccountID: &cash.ID, CategoryID: &cat, Amount: 300_00, Currency: "USD"},
		{Type: entities.TxTransfer, FromAccountID: &cash.ID, ToAccountID: &other, Amount: 200_00, Currency: "USD"},
		{Type: entities.TxTransfer, FromAccountID: &other, ToAccountID: &cash.ID, Amount: 100_00, Currency: "USD"},
	}

	// 1000 + 500 − 300 − 200 + 100 = 1100
	s.Equal(int64(1100_00), ledger.DeriveBalance(cash, txns))
}

func (s *LedgerSuite) TestDeriveBalance_Liability() {
	card := liability(uuid.New(), "USD", 0)
	bank := uuid.New()
	cat := uuid.New()

	txns := []entities.Transaction{
		// spend on the card → owe more
		{Type: entities.TxExpense, FromAccountID: &card.ID, CategoryID: &cat, Amount: 300_00, Currency: "USD"},
		// pay the bill from a bank account → owe less
		{Type: entities.TxTransfer, FromAccountID: &bank, ToAccountID: &card.ID, Amount: 100_00, Currency: "USD"},
	}

	// owed = 0 − 100 (repaid) + 300 (spent) = 200
	s.Equal(int64(200_00), ledger.DeriveBalance(card, txns))
}

func (s *LedgerSuite) TestDeriveBalance_CrossCurrencyTransfer() {
	usd := asset(uuid.New(), "USD", 1000_00)
	uzs := asset(uuid.New(), "UZS", 0)

	// 100.00 USD leaves usd, 1_250_000.00 UZS lands in uzs
	tx := entities.Transaction{
		Type:          entities.TxTransfer,
		FromAccountID: &usd.ID,
		ToAccountID:   &uzs.ID,
		Amount:        100_00,
		Currency:      "USD",
		ToAmount:      ptr(int64(1_250_000_00)),
		ToCurrency:    ptr("UZS"),
	}
	txns := []entities.Transaction{tx}

	s.Equal(int64(900_00), ledger.DeriveBalance(usd, txns), "from leg debits the primary amount")
	s.Equal(int64(1_250_000_00), ledger.DeriveBalance(uzs, txns), "to leg credits the second amount")
}

// --- rate freezing ----------------------------------------------------------

func (s *LedgerSuite) TestFreezeBase_RoundsHalfAwayFromZero() {
	s.Equal(int64(101), ledger.FreezeBase(100, fx.MustParseRate("1.005")), "100×1.005 = 100.5 → 101")
	s.Equal(int64(100), ledger.FreezeBase(100, fx.MustParseRate("1.004")), "100×1.004 = 100.4 → 100")
	s.Equal(int64(-2), ledger.FreezeBase(-3, fx.MustParseRate("0.5")), "-3×0.5 = -1.5 → -2")
	s.Equal(int64(1_250_050), ledger.FreezeBase(100, fx.MustParseRate("12500.50")), "exact, no float drift")
}

// --- net worth across currencies --------------------------------------------

func (s *LedgerSuite) TestNetWorth_MultiCurrency() {
	balances := []ledger.AccountBalance{
		{Account: asset(uuid.New(), "USD", 0), Balance: 1000_00},    // 1000.00 USD
		{Account: asset(uuid.New(), "EUR", 0), Balance: 500_00},     // 500.00 EUR @1.10 = 550.00
		{Account: liability(uuid.New(), "USD", 0), Balance: 200_00}, // owe 200.00 USD
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
		{Account: a, Balance: ledger.DeriveBalance(a, nil)},
		{Account: b, Balance: ledger.DeriveBalance(b, nil)},
	}
	before, err := ledger.NetWorth("USD", withoutTransfer, nil)
	s.Require().NoError(err)

	txns := []entities.Transaction{
		{Type: entities.TxTransfer, FromAccountID: &a.ID, ToAccountID: &b.ID, Amount: 300_00, Currency: "USD"},
	}
	withTransfer := []ledger.AccountBalance{
		{Account: a, Balance: ledger.DeriveBalance(a, txns)},
		{Account: b, Balance: ledger.DeriveBalance(b, txns)},
	}
	after, err := ledger.NetWorth("USD", withTransfer, nil)
	s.Require().NoError(err)

	s.Equal(before.Amount(), after.Amount(), "a transfer only moves value between accounts")
	s.Equal(int64(1000_00), after.Amount())
}

func (s *LedgerSuite) TestNetWorth_MissingRateErrors() {
	balances := []ledger.AccountBalance{
		{Account: asset(uuid.New(), "EUR", 0), Balance: 100_00},
	}

	_, err := ledger.NetWorth("USD", balances, nil)
	s.Require().Error(err)
	s.Contains(err.Error(), "EUR")
}
