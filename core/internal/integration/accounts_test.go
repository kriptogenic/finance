package integration

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"finance/internal/entities"
	accountrepository "finance/internal/repositories/account_repository"
	"finance/internal/ledger"
)

func TestAccount_CRUD(t *testing.T) {
	reset(t)
	ctx := context.Background()
	repo := accountRepo()

	acc := entities.Account{
		Name: "Cash", Kind: entities.KindAsset, Type: entities.TypeCash,
		Currency: "UZS", OpeningBalance: 500000,
	}
	require.NoError(t, repo.Create(ctx, &acc))
	require.NotEqual(t, uuid.Nil, acc.ID)
	require.False(t, acc.CreatedAt.IsZero())

	got, err := repo.Get(ctx, acc.ID)
	require.NoError(t, err)
	assert.Equal(t, "Cash", got.Name)
	assert.Equal(t, int64(500000), got.OpeningBalance)

	// update mutable fields
	got.Name = "Wallet"
	got.Archived = true
	require.NoError(t, repo.Update(ctx, got))

	got, err = repo.Get(ctx, acc.ID)
	require.NoError(t, err)
	assert.Equal(t, "Wallet", got.Name)
	assert.True(t, got.Archived)

	// archived hidden by default, shown when requested
	active, err := repo.List(ctx, false)
	require.NoError(t, err)
	assert.Empty(t, active)
	all, err := repo.List(ctx, true)
	require.NoError(t, err)
	assert.Len(t, all, 1)

	require.NoError(t, repo.Delete(ctx, acc.ID))
	_, err = repo.Get(ctx, acc.ID)
	assert.ErrorIs(t, err, accountrepository.ErrNotFound)
}

func TestAccount_NotFound(t *testing.T) {
	reset(t)
	ctx := context.Background()
	_, err := accountRepo().Get(ctx, uuid.New())
	assert.ErrorIs(t, err, accountrepository.ErrNotFound)
}

func TestAccount_KindTypeConstraint(t *testing.T) {
	reset(t)
	ctx := context.Background()
	// liability kind with an asset type violates accounts_kind_type_chk
	bad := entities.Account{Name: "x", Kind: entities.KindLiability, Type: entities.TypeCash, Currency: "UZS"}
	err := accountRepo().Create(ctx, &bad)
	require.Error(t, err)
}

func TestAccount_DeleteInUse(t *testing.T) {
	reset(t)
	ctx := context.Background()

	cash := mustAccount(t, assetAccount("UZS", 0))
	cat := mustCategory(t, entities.Category{Name: "Food", Type: entities.CategoryExpense})

	tx := buildExpense(t, cash, cat, 100, "UZS")
	require.NoError(t, transactionRepo().Create(ctx, &tx))

	err := accountRepo().Delete(ctx, cash.ID)
	assert.ErrorIs(t, err, accountrepository.ErrInUse, "account with transactions cannot be hard-deleted")
}

func TestAccount_BalancesDerived(t *testing.T) {
	reset(t)
	ctx := context.Background()

	cash := mustAccount(t, assetAccount("UZS", 1000000))   // opening 1,000,000
	card := mustAccount(t, liabilityAccount("UZS", 0))     // credit card
	usd := mustAccount(t, assetAccount("USD", 0))          // for cross-currency
	food := mustCategory(t, entities.Category{Name: "Food", Type: entities.CategoryExpense})
	salary := mustCategory(t, entities.Category{Name: "Salary", Type: entities.CategoryIncome})

	repo := transactionRepo()
	// +500000 income to cash
	inc := buildIncome(t, cash, salary, 500000, "UZS")
	require.NoError(t, repo.Create(ctx, &inc))
	// -120000 expense from cash
	exp := buildExpense(t, cash, food, 120000, "UZS")
	require.NoError(t, repo.Create(ctx, &exp))
	// spend 80000 on the card (liability owed increases)
	cardSpend := buildExpense(t, card, food, 80000, "UZS")
	require.NoError(t, repo.Create(ctx, &cardSpend))
	// cross-currency transfer: 125000 UZS -> 10.00 USD
	xfer := buildTransfer(t, cash, usd, 125000, ptr(int64(1000)), "UZS")
	require.NoError(t, repo.Create(ctx, &xfer))

	balances, err := accountRepo().Balances(ctx)
	require.NoError(t, err)

	// cash = 1,000,000 + 500,000 - 120,000 - 125,000
	assert.Equal(t, int64(1255000), balances[cash.ID])
	// card owed = spent 80,000
	assert.Equal(t, int64(80000), balances[card.ID])
	// usd credited the second leg
	assert.Equal(t, int64(1000), balances[usd.ID])

	// cross-check against the pure ledger derivation
	accs, _ := accountRepo().List(ctx, true)
	txns, _ := repo.List(ctx, allFilter())
	for _, a := range accs {
		assert.Equal(t, ledger.DeriveBalance(a, txns), balances[a.ID], "account %s", a.Name)
	}
}
