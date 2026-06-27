package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"finance/internal/entities"
	receiptrepository "finance/internal/repositories/receipt_repository"
)

func receiptRepo() receiptrepository.Repository { return receiptrepository.NewRepository(testDB) }

// mustReceipt inserts a pending receipt and returns its id.
func mustReceipt(t *testing.T) entities.Receipt {
	t.Helper()
	rec := entities.Receipt{QRURL: "https://ofd.soliq.uz/check?x=1", Status: entities.ReceiptPending}
	require.NoError(t, receiptRepo().Create(context.Background(), &rec))

	return rec
}

func TestReceiptAutoLinkCandidate(t *testing.T) {
	reset(t)
	ctx := context.Background()
	repo := receiptRepo()

	cash := mustAccount(t, assetAccount("UZS", 0))
	food := mustCategory(t, entities.Category{Name: "Food", Type: entities.CategoryExpense})

	exp := buildExpense(t, cash, food, 50000, "UZS")
	require.NoError(t, transactionRepo().Create(ctx, &exp))

	now := time.Now()
	window := func() (time.Time, time.Time) { return now.Add(-time.Hour), now.Add(time.Hour) }

	// exact single match -> returns the transaction id
	from, to := window()
	got, err := repo.FindAutoLinkCandidate(ctx, 50000, from, to)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, exp.ID, *got)

	// wrong amount -> no match
	got, err = repo.FindAutoLinkCandidate(ctx, 999, from, to)
	require.NoError(t, err)
	assert.Nil(t, got)

	// once linked, the expense is excluded from future candidates
	rec := mustReceipt(t)
	require.NoError(t, repo.SetTransaction(ctx, rec.ID, &exp.ID))
	got, err = repo.FindAutoLinkCandidate(ctx, 50000, from, to)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestReceiptAutoLinkAmbiguous(t *testing.T) {
	reset(t)
	ctx := context.Background()
	repo := receiptRepo()

	cash := mustAccount(t, assetAccount("UZS", 0))
	food := mustCategory(t, entities.Category{Name: "Food", Type: entities.CategoryExpense})

	for range 2 {
		exp := buildExpense(t, cash, food, 50000, "UZS")
		require.NoError(t, transactionRepo().Create(ctx, &exp))
	}

	now := time.Now()
	got, err := repo.FindAutoLinkCandidate(ctx, 50000, now.Add(-time.Hour), now.Add(time.Hour))
	require.NoError(t, err)
	assert.Nil(t, got) // two matches -> ambiguous, skip
}

func TestReceiptSetTransactionConflict(t *testing.T) {
	reset(t)
	ctx := context.Background()
	repo := receiptRepo()

	cash := mustAccount(t, assetAccount("UZS", 0))
	food := mustCategory(t, entities.Category{Name: "Food", Type: entities.CategoryExpense})
	exp := buildExpense(t, cash, food, 50000, "UZS")
	require.NoError(t, transactionRepo().Create(ctx, &exp))

	first := mustReceipt(t)
	require.NoError(t, repo.SetTransaction(ctx, first.ID, &exp.ID))

	// a second receipt cannot claim the same transaction (1:1)
	second := mustReceipt(t)
	err := repo.SetTransaction(ctx, second.ID, &exp.ID)
	assert.ErrorIs(t, err, receiptrepository.ErrAlreadyLinked)

	// unlinking the first frees it again
	require.NoError(t, repo.SetTransaction(ctx, first.ID, nil))
	require.NoError(t, repo.SetTransaction(ctx, second.ID, &exp.ID))

	got, err := repo.Get(ctx, second.ID)
	require.NoError(t, err)
	require.NotNil(t, got.TransactionID)
	assert.Equal(t, exp.ID, *got.TransactionID)
}
