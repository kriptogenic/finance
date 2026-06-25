package integration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"finance/internal/entities"
	"finance/internal/ledger"
	splitrepository "finance/internal/repositories/split_repository"
)

func splitRepo() splitrepository.Repository { return splitrepository.NewRepository(testDB) }

func TestSplitExpense(t *testing.T) {
	reset(t)
	ctx := context.Background()

	paying := mustAccount(t, assetAccount("UZS", 1_000))
	cat := mustCategory(t, entities.Category{Name: "Food", Type: entities.CategoryExpense})

	// a 200 bill paid in full from the paying account
	expense := buildExpense(t, paying, cat, 200, "UZS")
	require.NoError(t, transactionRepo().Create(ctx, &expense))

	// split 4 ways: you 50, three friends 50 each
	parts := []ledger.SplitParticipant{
		{Name: "Alice", Amount: 50},
		{Name: "Bob", Amount: 50},
		{Name: "Cara", Amount: 50},
	}
	group, err := splitRepo().Apply(ctx, splitrepository.ApplyParams{
		MainTxID: expense.ID, PayingAccount: paying, MyShare: 50, Participants: parts,
	})
	require.NoError(t, err)
	require.NotNil(t, group)

	// main expense shrank to your share
	main, err := transactionRepo().Get(ctx, expense.ID)
	require.NoError(t, err)
	require.Equal(t, int64(50), main.Amount)
	require.NotNil(t, main.SplitGroupID)

	// four legs in the group (expense + 3 transfers)
	legs, err := transactionRepo().ListBySplitGroup(ctx, *group)
	require.NoError(t, err)
	require.Len(t, legs, 4)
	require.Equal(t, entities.TxExpense, legs[0].Type) // expense first

	// three receivable accounts now exist, each owed their share
	accs, err := accountRepo().List(ctx, true)
	require.NoError(t, err)
	var receivables int
	for _, a := range accs {
		if a.Type == entities.TypeReceivable {
			receivables++
		}
	}
	require.Equal(t, 3, receivables)

	balances, err := accountRepo().Balances(ctx)
	require.NoError(t, err)
	// paying account lost the full 200 (your 50 + 3×50)
	require.Equal(t, int64(800), balances[paying.ID])
	// net worth only dropped by your share (assets: 800 cash + 150 owed = 950)
	var assets int64
	for id, b := range balances {
		acc := accByID(t, accs, id)
		if acc.IsAsset() {
			assets += b
		}
	}
	require.Equal(t, int64(950), assets)

	// a friend repays in full → their person account auto-archives on settle
	alice := personAccount(t, accs, "Alice")
	repay := buildTransfer(t, alice, paying, 50, nil, "UZS")
	require.NoError(t, transactionRepo().Create(ctx, &repay))
	require.NoError(t, accountRepo().SettleReceivables(ctx))

	got, err := accountRepo().Get(ctx, alice.ID)
	require.NoError(t, err)
	require.True(t, got.Archived, "fully-repaid person should be archived")

	// un-split restores the full expense and drops the (unpaid) person accounts
	_, err = splitRepo().Apply(ctx, splitrepository.ApplyParams{
		MainTxID: expense.ID, PayingAccount: paying, MyShare: 200, Participants: nil,
	})
	require.NoError(t, err)

	main, err = transactionRepo().Get(ctx, expense.ID)
	require.NoError(t, err)
	require.Equal(t, int64(200), main.Amount)
	require.Nil(t, main.SplitGroupID)

	// Alice survives (a repayment still references her); Bob and Cara are gone
	accs, err = accountRepo().List(ctx, true)
	require.NoError(t, err)
	require.Nil(t, findPerson(accs, "Bob"))
	require.Nil(t, findPerson(accs, "Cara"))
}

func accByID(t *testing.T, accs []entities.Account, id interface{ String() string }) entities.Account {
	t.Helper()
	for _, a := range accs {
		if a.ID.String() == id.String() {
			return a
		}
	}
	t.Fatalf("account %s not found", id)

	return entities.Account{}
}

func personAccount(t *testing.T, accs []entities.Account, name string) entities.Account {
	t.Helper()
	a := findPerson(accs, name)
	require.NotNil(t, a, "person %s not found", name)

	return *a
}

func findPerson(accs []entities.Account, name string) *entities.Account {
	for i := range accs {
		if accs[i].Type == entities.TypeReceivable && accs[i].Name == name {
			return &accs[i]
		}
	}

	return nil
}
