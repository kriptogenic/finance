package integration

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
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

func TestReceiptFiscalDedup(t *testing.T) {
	reset(t)
	ctx := context.Background()
	repo := receiptRepo()

	term, sign := "TERM1", "SIGN1"
	seq := 42
	fiscal := func() entities.Receipt {
		return entities.Receipt{
			QRURL:      "https://ofd.soliq.uz/check?t=TERM1&r=42&s=SIGN1",
			Status:     entities.ReceiptPending,
			TerminalID: &term,
			ReceiptSeq: &seq,
			FiscalSign: &sign,
		}
	}

	// none stored yet
	_, err := repo.FindByFiscal(ctx, &term, &seq, &sign)
	assert.ErrorIs(t, err, receiptrepository.ErrNotFound)

	first := fiscal()
	require.NoError(t, repo.Create(ctx, &first))

	// same fiscal triple -> unique index rejects with ErrDuplicate
	dup := fiscal()
	assert.ErrorIs(t, repo.Create(ctx, &dup), receiptrepository.ErrDuplicate)

	// lookup returns the original receipt
	got, err := repo.FindByFiscal(ctx, &term, &seq, &sign)
	require.NoError(t, err)
	assert.Equal(t, first.ID, got.ID)

	// rows missing part of the triple are unconstrained (manual/partial scans)
	require.NoError(t, repo.Create(ctx, &entities.Receipt{QRURL: "https://x/1", Status: entities.ReceiptPending}))
	require.NoError(t, repo.Create(ctx, &entities.Receipt{QRURL: "https://x/2", Status: entities.ReceiptPending}))
}

func TestReceiptRawPayload(t *testing.T) {
	reset(t)
	ctx := context.Background()
	repo := receiptRepo()

	rec := mustReceipt(t)

	// nothing stored yet
	payload, err := repo.GetRawPayload(ctx, rec.ID)
	require.NoError(t, err)
	assert.Empty(t, payload)

	require.NoError(t, repo.SetRawPayload(ctx, rec.ID, `{"ok":true}`))
	payload, err = repo.GetRawPayload(ctx, rec.ID)
	require.NoError(t, err)
	assert.Equal(t, `{"ok":true}`, payload)

	_, err = repo.GetRawPayload(ctx, uuid.New())
	assert.ErrorIs(t, err, receiptrepository.ErrNotFound)
}

func TestReceiptDelete(t *testing.T) {
	reset(t)
	ctx := context.Background()
	repo := receiptRepo()

	rec := mustReceipt(t)

	// deleting removes the receipt (and its items, via ON DELETE CASCADE)
	require.NoError(t, repo.Delete(ctx, rec.ID))
	_, err := repo.Get(ctx, rec.ID)
	assert.ErrorIs(t, err, receiptrepository.ErrNotFound)

	// deleting a missing receipt reports not found
	assert.ErrorIs(t, repo.Delete(ctx, uuid.New()), receiptrepository.ErrNotFound)
}

// TestTransactionCarriesReceiptID covers the reverse lookup: once a receipt is
// linked, the transaction read exposes the receipt id (correlated subquery).
func TestTransactionCarriesReceiptID(t *testing.T) {
	reset(t)
	ctx := context.Background()

	cash := mustAccount(t, assetAccount("UZS", 0))
	food := mustCategory(t, entities.Category{Name: "Food", Type: entities.CategoryExpense})
	exp := buildExpense(t, cash, food, 50000, "UZS")
	require.NoError(t, transactionRepo().Create(ctx, &exp))

	// before linking: no receipt
	got, err := transactionRepo().Get(ctx, exp.ID)
	require.NoError(t, err)
	assert.Nil(t, got.ReceiptID)

	rec := mustReceipt(t)
	require.NoError(t, receiptRepo().SetTransaction(ctx, rec.ID, &exp.ID))

	// after linking: the transaction points back at the receipt
	got, err = transactionRepo().Get(ctx, exp.ID)
	require.NoError(t, err)
	require.NotNil(t, got.ReceiptID)
	assert.Equal(t, rec.ID, *got.ReceiptID)
}
