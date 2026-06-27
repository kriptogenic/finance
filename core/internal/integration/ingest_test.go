package integration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"finance/internal/entities"
)

func TestIngest_Idempotent(t *testing.T) {
	reset(t)
	ctx := context.Background()
	repo := transactionRepo()

	cash := mustAccount(t, assetAccount("UZS", 0))
	food := mustCategory(t, entities.Category{Name: "Food", Type: entities.CategoryExpense})

	// first ingest creates
	tx1 := buildExpense(t, cash, food, 57550_00, "UZS")
	tx1.ExternalID = ptr("tg:1:100")
	created, err := repo.Ingest(ctx, &tx1)
	require.NoError(t, err)
	assert.True(t, created)
	require.NotEqual(t, "", tx1.ID.String())

	// second ingest with the same external_id is a no-op returning the original
	tx2 := buildExpense(t, cash, food, 999_00, "UZS") // different amount, same key
	tx2.ExternalID = ptr("tg:1:100")
	created, err = repo.Ingest(ctx, &tx2)
	require.NoError(t, err)
	assert.False(t, created, "same external_id must dedupe")
	assert.Equal(t, tx1.ID, tx2.ID, "returns the originally-stored transaction")
	assert.Equal(t, int64(57550_00), tx2.Amount.Minor(), "original amount preserved, not overwritten")

	// exactly one row exists
	all, err := repo.List(ctx, allFilter())
	require.NoError(t, err)
	assert.Len(t, all, 1)
}

func TestIngest_RequiresExternalID(t *testing.T) {
	reset(t)
	ctx := context.Background()

	cash := mustAccount(t, assetAccount("UZS", 0))
	food := mustCategory(t, entities.Category{Name: "Food", Type: entities.CategoryExpense})

	tx := buildExpense(t, cash, food, 100, "UZS") // no ExternalID
	_, err := transactionRepo().Ingest(ctx, &tx)
	require.Error(t, err)
}
