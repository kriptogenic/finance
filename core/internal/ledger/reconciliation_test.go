package ledger_test

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"finance/internal/entities"
	"finance/internal/ledger"
)

func cardAccount(id uuid.UUID, currency, last4 string) entities.Account {
	a := asset(id, currency, 0)
	a.Type = entities.TypeDebitCard
	a.CardLast4 = ptr(last4)
	return a
}

func snapshot(last4, currency string, amount int64) entities.BalanceSnapshot {
	return entities.BalanceSnapshot{CardLast4: last4, Currency: currency, Amount: amount}
}

func (s *LedgerSuite) TestReconcile_InSync() {
	acc := cardAccount(uuid.New(), "UZS", "8400")
	rows := ledger.Reconcile(
		[]entities.BalanceSnapshot{snapshot("8400", "UZS", 692446)},
		[]entities.Account{acc},
		map[uuid.UUID]int64{acc.ID: 692446},
	)

	require.Len(s.T(), rows, 1)
	require.True(s.T(), rows[0].CurrencyMatch)
	require.True(s.T(), rows[0].InSync)
	require.EqualValues(s.T(), 0, rows[0].Delta)
}

func (s *LedgerSuite) TestReconcile_OffByDelta() {
	acc := cardAccount(uuid.New(), "UZS", "7351")
	rows := ledger.Reconcile(
		[]entities.BalanceSnapshot{snapshot("7351", "UZS", 96020)},
		[]entities.Account{acc},
		map[uuid.UUID]int64{acc.ID: 90000},
	)

	require.Len(s.T(), rows, 1)
	require.True(s.T(), rows[0].CurrencyMatch)
	require.False(s.T(), rows[0].InSync)
	require.EqualValues(s.T(), 6020, rows[0].Delta) // reported − derived
}

func (s *LedgerSuite) TestReconcile_CurrencyMismatch() {
	acc := cardAccount(uuid.New(), "USD", "4853")
	rows := ledger.Reconcile(
		[]entities.BalanceSnapshot{snapshot("4853", "UZS", 6986)},
		[]entities.Account{acc},
		map[uuid.UUID]int64{acc.ID: 6986},
	)

	require.Len(s.T(), rows, 1)
	require.False(s.T(), rows[0].CurrencyMatch)
	require.False(s.T(), rows[0].InSync)
	require.EqualValues(s.T(), 0, rows[0].Delta) // not meaningful, left zero
}

func (s *LedgerSuite) TestReconcile_OnlyMatchedPairs() {
	withCard := cardAccount(uuid.New(), "UZS", "2953")
	noCard := asset(uuid.New(), "UZS", 0) // no card_last4 → never a row
	rows := ledger.Reconcile(
		[]entities.BalanceSnapshot{
			snapshot("2953", "UZS", 0),
			snapshot("9999", "UZS", 100), // no matching account → dropped
		},
		[]entities.Account{withCard, noCard},
		map[uuid.UUID]int64{withCard.ID: 0},
	)

	require.Len(s.T(), rows, 1)
	require.Equal(s.T(), "2953", *rows[0].Account.CardLast4)
}
