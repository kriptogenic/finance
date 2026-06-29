package ledger_test

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"finance/internal/entities"
	"finance/internal/ledger"
	"finance/pkg/money"
)

func cardAccount(id uuid.UUID, currency, last4 string) entities.Account {
	a := asset(id, currency, 0)
	a.Type = entities.TypeDebitCard
	a.CardLast4 = ptr(last4)
	return a
}

func snapshot(last4, currency string, amount int64) entities.BalanceSnapshot {
	return entities.BalanceSnapshot{CardLast4: last4, Amount: money.New(amount, currency)}
}

func (s *LedgerSuite) TestReconcile_InSync() {
	acc := cardAccount(uuid.New(), "UZS", "8400")
	rows := ledger.Reconcile(
		[]entities.BalanceSnapshot{snapshot("8400", "UZS", 692446)},
		[]entities.Account{acc},
		map[uuid.UUID]money.Money{acc.ID: money.New(692446, "UZS")},
	)

	require.Len(s.T(), rows, 1)
	require.True(s.T(), rows[0].CurrencyMatch)
	require.True(s.T(), rows[0].InSync)
	require.EqualValues(s.T(), 0, rows[0].Delta.Minor())
}

func (s *LedgerSuite) TestReconcile_OffByDelta() {
	acc := cardAccount(uuid.New(), "UZS", "7351")
	rows := ledger.Reconcile(
		[]entities.BalanceSnapshot{snapshot("7351", "UZS", 96020)},
		[]entities.Account{acc},
		map[uuid.UUID]money.Money{acc.ID: money.New(90000, "UZS")},
	)

	require.Len(s.T(), rows, 1)
	require.True(s.T(), rows[0].CurrencyMatch)
	require.False(s.T(), rows[0].InSync)
	require.EqualValues(s.T(), 6020, rows[0].Delta.Minor()) // reported − derived
}

func (s *LedgerSuite) TestReconcile_CurrencyMismatch() {
	acc := cardAccount(uuid.New(), "USD", "4853")
	rows := ledger.Reconcile(
		[]entities.BalanceSnapshot{snapshot("4853", "UZS", 6986)},
		[]entities.Account{acc},
		map[uuid.UUID]money.Money{acc.ID: money.New(6986, "USD")},
	)

	require.Len(s.T(), rows, 1)
	require.False(s.T(), rows[0].CurrencyMatch)
	require.False(s.T(), rows[0].InSync)
	require.True(s.T(), rows[0].Delta.IsZeroValue()) // not meaningful, left zero
}

func creditCard(id uuid.UUID, currency, last4 string, limit int64) entities.Account {
	a := liability(id, currency, 0)
	a.CardLast4 = ptr(last4)
	a.CreditLimit = ptr(money.New(limit, currency))
	return a
}

// The bank reports available credit (limit − owed), so reconciliation compares
// against credit_limit minus the derived owed balance, not the owed amount itself.
func (s *LedgerSuite) TestReconcile_CreditCardUsesAvailableCredit() {
	acc := creditCard(uuid.New(), "UZS", "1234", 10_000_00)
	owed := money.New(3_000_00, "UZS") // derived liability balance = owed

	rows := ledger.Reconcile(
		[]entities.BalanceSnapshot{snapshot("1234", "UZS", 7_000_00)}, // available = 10,000 − 3,000
		[]entities.Account{acc},
		map[uuid.UUID]money.Money{acc.ID: owed},
	)

	require.Len(s.T(), rows, 1)
	require.True(s.T(), rows[0].InSync)
	require.EqualValues(s.T(), 7_000_00, rows[0].Derived.Minor()) // shown as available credit
	require.EqualValues(s.T(), 0, rows[0].Delta.Minor())
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
		map[uuid.UUID]money.Money{withCard.ID: money.New(0, "UZS")},
	)

	require.Len(s.T(), rows, 1)
	require.Equal(s.T(), "2953", *rows[0].Account.CardLast4)
}
