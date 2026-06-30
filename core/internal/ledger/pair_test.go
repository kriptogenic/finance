package ledger_test

import (
	"time"

	"github.com/google/uuid"

	"finance/internal/entities"
	"finance/internal/ledger"
	"finance/pkg/money"
)

func expenseLeg(account uuid.UUID, amount int64, currency, extID string, when time.Time) entities.Transaction {
	cat := uuid.New()

	return entities.Transaction{
		ID: uuid.New(), Date: when, Type: entities.TxExpense,
		FromAccountID: &account, CategoryID: &cat,
		Amount: money.New(amount, currency), ExternalID: &extID, Tags: []string{"humo"},
	}
}

func incomeLeg(account uuid.UUID, amount int64, currency, extID string, when time.Time) entities.Transaction {
	cat := uuid.New()

	return entities.Transaction{
		ID: uuid.New(), Date: when, Type: entities.TxIncome,
		ToAccountID: &account, CategoryID: &cat,
		Amount: money.New(amount, currency), ExternalID: &extID, Tags: []string{"humo"},
	}
}

func (s *LedgerSuite) TestTransferFromLegs_PairsOppositeLegs() {
	visa, humo := uuid.New(), uuid.New()
	t0 := time.Date(2026, 6, 14, 9, 36, 0, 0, time.UTC)
	debit := expenseLeg(visa, 1_000_000, "UZS", "tg:1:10", t0.Add(20*time.Second))
	credit := incomeLeg(humo, 1_000_000, "UZS", "sms:abc", t0)

	tx, err := ledger.TransferFromLegs(credit, debit) // order should not matter
	s.Require().NoError(err)

	s.Equal(entities.TxTransfer, tx.Type)
	s.Equal(&visa, tx.FromAccountID)
	s.Equal(&humo, tx.ToAccountID)
	s.Nil(tx.CategoryID, "a transfer carries no category")
	s.Equal(int64(1_000_000), tx.Amount.Minor())
	s.Equal(t0, tx.Date, "uses the earlier leg's time")
	s.Require().NotNil(tx.TransferGroupID)
	s.Equal("tf:sms:abc|tg:1:10", *tx.TransferGroupID, "group id is order-independent")
	s.ElementsMatch([]string{"humo"}, tx.Tags)
}

func (s *LedgerSuite) TestTransferFromLegs_RejectsSameDirection() {
	_, err := ledger.TransferFromLegs(
		expenseLeg(uuid.New(), 100, "UZS", "a", time.Now()),
		expenseLeg(uuid.New(), 100, "UZS", "b", time.Now()),
	)
	s.Error(err)
}

func (s *LedgerSuite) TestTransferFromLegs_RejectsSameAccount() {
	acc := uuid.New()
	_, err := ledger.TransferFromLegs(
		expenseLeg(acc, 100, "UZS", "a", time.Now()),
		incomeLeg(acc, 100, "UZS", "b", time.Now()),
	)
	s.Error(err)
}

func (s *LedgerSuite) TestTransferFromLegs_RejectsAmountMismatch() {
	_, err := ledger.TransferFromLegs(
		expenseLeg(uuid.New(), 100, "UZS", "a", time.Now()),
		incomeLeg(uuid.New(), 101, "UZS", "b", time.Now()),
	)
	s.Error(err)
}

func (s *LedgerSuite) TestTransferFromLegs_RejectsCurrencyMismatch() {
	_, err := ledger.TransferFromLegs(
		expenseLeg(uuid.New(), 100, "USD", "a", time.Now()),
		incomeLeg(uuid.New(), 100, "UZS", "b", time.Now()),
	)
	s.Error(err)
}
