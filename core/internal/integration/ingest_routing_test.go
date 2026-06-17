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

func TestCategory_ResolveForIngest_CreatesMissingBucket(t *testing.T) {
	reset(t)
	ctx := context.Background()
	repo := categoryRepo()

	// no income bucket seeded — resolving should create it on demand
	got, err := repo.ResolveForIngest(ctx, entities.CategoryIncome, "SALARY")
	require.NoError(t, err)

	created, err := repo.Get(ctx, got)
	require.NoError(t, err)
	assert.Equal(t, entities.CategoryIncome, created.Type)
	require.NotNil(t, created.SystemKey)
	assert.Equal(t, "uncategorized_income", *created.SystemKey)

	// a second call reuses the same bucket rather than creating a duplicate
	again, err := repo.ResolveForIngest(ctx, entities.CategoryIncome, "BONUS")
	require.NoError(t, err)
	assert.Equal(t, got, again, "reuses the existing bucket")
}
