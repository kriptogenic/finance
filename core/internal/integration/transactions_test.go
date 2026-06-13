package integration

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"finance/internal/entities"
	"finance/internal/ledger"
	transactionrepository "finance/internal/repositories/transaction_repository"
	"finance/pkg/fx"
)

func TestTransaction_CreateGetDelete(t *testing.T) {
	reset(t)
	ctx := context.Background()
	repo := transactionRepo()

	cash := mustAccount(t, assetAccount("UZS", 0))
	food := mustCategory(t, entities.Category{Name: "Food", Type: entities.CategoryExpense})

	tx := buildExpense(t, cash, food, 120000, "UZS")
	tx.Note = ptr("lunch")
	tx.Tags = []string{"work"}
	require.NoError(t, repo.Create(ctx, &tx))
	require.NotEqual(t, uuid.Nil, tx.ID)

	got, err := repo.Get(ctx, tx.ID)
	require.NoError(t, err)
	assert.Equal(t, entities.TxExpense, got.Type)
	assert.Equal(t, int64(120000), got.Amount)
	assert.Equal(t, "UZS", got.Currency)
	require.NotNil(t, got.Note)
	assert.Equal(t, "lunch", *got.Note)
	assert.Equal(t, []string{"work"}, got.Tags)

	require.NoError(t, repo.Delete(ctx, tx.ID))
	_, err = repo.Get(ctx, tx.ID)
	assert.ErrorIs(t, err, transactionrepository.ErrNotFound)
}

func TestTransaction_RateFrozenRoundTrip(t *testing.T) {
	reset(t)
	ctx := context.Background()
	repo := transactionRepo()

	usd := mustAccount(t, assetAccount("USD", 0))
	food := mustCategory(t, entities.Category{Name: "Food", Type: entities.CategoryExpense})

	// USD expense with base UZS: build with a frozen rate
	rate := fx.MustParseRate("12500")
	tx := mustBuild(t, ledger.NewTransaction{
		Type: entities.TxExpense, From: &usd, Category: &food, Amount: 1000, RateToBase: &rate,
	}, "UZS")
	require.NoError(t, repo.Create(ctx, &tx))

	got, err := repo.Get(ctx, tx.ID)
	require.NoError(t, err)
	require.NotNil(t, got.RateToBase)
	require.NotNil(t, got.BaseAmount)
	assert.Equal(t, "12500.0000000000", got.RateToBase.String())
	assert.Equal(t, int64(12500000), *got.BaseAmount) // 1000 * 12500
}

func TestTransaction_Update(t *testing.T) {
	reset(t)
	ctx := context.Background()
	repo := transactionRepo()

	cash := mustAccount(t, assetAccount("UZS", 0))
	food := mustCategory(t, entities.Category{Name: "Food", Type: entities.CategoryExpense})
	rent := mustCategory(t, entities.Category{Name: "Rent", Type: entities.CategoryExpense})

	tx := buildExpense(t, cash, food, 100000, "UZS")
	require.NoError(t, repo.Create(ctx, &tx))

	// full replace: change amount and category, keep id
	updated := buildExpense(t, cash, rent, 250000, "UZS")
	updated.ID = tx.ID
	require.NoError(t, repo.Update(ctx, &updated))

	got, err := repo.Get(ctx, tx.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(250000), got.Amount)
	require.NotNil(t, got.CategoryID)
	assert.Equal(t, rent.ID, *got.CategoryID)
}

func TestTransaction_ShapeConstraint(t *testing.T) {
	reset(t)
	ctx := context.Background()

	cash := mustAccount(t, assetAccount("UZS", 0))
	other := mustAccount(t, assetAccount("UZS", 0))
	food := mustCategory(t, entities.Category{Name: "Food", Type: entities.CategoryExpense})

	// a transfer carrying a category bypasses the engine but must be rejected by
	// the DB's transactions_shape_chk
	bad := entities.Transaction{
		Date: time.Now(), Type: entities.TxTransfer,
		FromAccountID: &cash.ID, ToAccountID: &other.ID, CategoryID: &food.ID,
		Amount: 100, Currency: "UZS", Tags: []string{},
	}
	require.Error(t, transactionRepo().Create(ctx, &bad))
}

func TestTransaction_Filters(t *testing.T) {
	reset(t)
	ctx := context.Background()
	repo := transactionRepo()

	cash := mustAccount(t, assetAccount("UZS", 0))
	usd := mustAccount(t, assetAccount("USD", 0))
	food := mustCategory(t, entities.Category{Name: "Food", Type: entities.CategoryExpense})
	salary := mustCategory(t, entities.Category{Name: "Salary", Type: entities.CategoryIncome})

	may := time.Date(2026, time.May, 10, 12, 0, 0, 0, time.UTC)
	jun := time.Date(2026, time.June, 10, 12, 0, 0, 0, time.UTC)

	exp := buildExpense(t, cash, food, 50000, "UZS")
	exp.Date = may
	exp.Note = ptr("groceries run")
	exp.Tags = []string{"weekly"}
	require.NoError(t, repo.Create(ctx, &exp))

	inc := buildIncome(t, cash, salary, 500000, "UZS")
	inc.Date = jun
	require.NoError(t, repo.Create(ctx, &inc))

	xfer := buildTransfer(t, cash, usd, 125000, ptr(int64(1000)), "UZS")
	xfer.Date = jun
	require.NoError(t, repo.Create(ctx, &xfer))

	count := func(f transactionrepository.Filter) int {
		f.Limit = 1000
		out, err := repo.List(ctx, f)
		require.NoError(t, err)

		return len(out)
	}

	assert.Equal(t, 3, count(transactionrepository.Filter{}))
	assert.Equal(t, 1, count(transactionrepository.Filter{Type: ptr(entities.TxExpense)}))
	assert.Equal(t, 1, count(transactionrepository.Filter{Type: ptr(entities.TxIncome)}))
	assert.Equal(t, 1, count(transactionrepository.Filter{CategoryID: &food.ID}))
	assert.Equal(t, 3, count(transactionrepository.Filter{AccountID: &cash.ID}), "cash is in all three (either leg)")
	assert.Equal(t, 1, count(transactionrepository.Filter{AccountID: &usd.ID}), "usd only the transfer to-leg")
	assert.Equal(t, 1, count(transactionrepository.Filter{Tag: ptr("weekly")}))
	assert.Equal(t, 1, count(transactionrepository.Filter{Query: ptr("groc")}))
	// June window catches the income and the transfer (both dated June 10), not the May expense
	assert.Equal(t, 2, count(transactionrepository.Filter{DateFrom: ptr(time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)), DateTo: ptr(jun.Add(time.Hour))}))
}
