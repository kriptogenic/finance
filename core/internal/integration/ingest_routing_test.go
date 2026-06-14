package integration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"finance/internal/entities"
	accountrepository "finance/internal/repositories/account_repository"
)

func TestAccount_ByCardLast4(t *testing.T) {
	reset(t)
	ctx := context.Background()

	acc := assetAccount("UZS", 0)
	acc.Type = entities.TypeDebitCard
	acc.CardLast4 = ptr("4853")
	created := mustAccount(t, acc)

	got, err := accountRepo().ByCardLast4(ctx, "4853")
	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)

	_, err = accountRepo().ByCardLast4(ctx, "0000")
	assert.ErrorIs(t, err, accountrepository.ErrNotFound)
}

func TestCategory_ResolveForIngest(t *testing.T) {
	reset(t)
	ctx := context.Background()
	repo := categoryRepo()

	// the default bucket is found by system_key
	def := entities.Category{Name: "Uncategorized", Type: entities.CategoryExpense, SystemKey: ptr("uncategorized_expense")}
	require.NoError(t, repo.Create(ctx, &def))

	got, err := repo.ResolveForIngest(ctx, entities.CategoryExpense, "RANDOM MERCHANT")
	require.NoError(t, err)
	assert.Equal(t, def.ID, got, "falls back to the Uncategorized bucket")

	// a merchant rule takes precedence
	groceries := mustCategory(t, entities.Category{Name: "Groceries", Type: entities.CategoryExpense})
	_, err = testDB.Pool.Exec(ctx,
		`INSERT INTO category_rules (pattern, category_id) VALUES ($1, $2)`, "HAVAS", groceries.ID)
	require.NoError(t, err)

	got, err = repo.ResolveForIngest(ctx, entities.CategoryExpense, "SP OOO HAVAS FOOD>T")
	require.NoError(t, err)
	assert.Equal(t, groceries.ID, got, "matching rule wins over the default")
}
