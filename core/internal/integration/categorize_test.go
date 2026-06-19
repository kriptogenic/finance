package integration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"finance/internal/entities"
	categoryrulerepository "finance/internal/repositories/category_rule_repository"
	transactionrepository "finance/internal/repositories/transaction_repository"
)

func TestTransaction_UncategorizedFilter(t *testing.T) {
	reset(t)
	ctx := context.Background()
	repo := transactionRepo()

	cash := mustAccount(t, assetAccount("UZS", 0))
	bucket := mustCategory(t, entities.Category{
		Name: "Uncategorized", Type: entities.CategoryExpense, SystemKey: ptr("uncategorized_expense"),
	})
	food := mustCategory(t, entities.Category{Name: "Food", Type: entities.CategoryExpense})
	salary := mustCategory(t, entities.Category{Name: "Salary", Type: entities.CategoryIncome})

	uncat := buildExpense(t, cash, bucket, 100_00, "UZS")
	require.NoError(t, repo.Create(ctx, &uncat))
	categorizedExpense := buildExpense(t, cash, food, 200_00, "UZS")
	require.NoError(t, repo.Create(ctx, &categorizedExpense))
	categorizedIncome := buildIncome(t, cash, salary, 300_00, "UZS")
	require.NoError(t, repo.Create(ctx, &categorizedIncome))

	got, err := repo.List(ctx, transactionrepository.Filter{Uncategorized: true, Limit: 100})
	require.NoError(t, err)
	require.Len(t, got, 1, "only the bucketed transaction is uncategorized")
	assert.Equal(t, uncat.ID, got[0].ID)
}

func TestTransaction_SetCategory(t *testing.T) {
	reset(t)
	ctx := context.Background()
	repo := transactionRepo()

	cash := mustAccount(t, assetAccount("UZS", 0))
	bucket := mustCategory(t, entities.Category{
		Name: "Uncategorized", Type: entities.CategoryExpense, SystemKey: ptr("uncategorized_expense"),
	})
	food := mustCategory(t, entities.Category{Name: "Food", Type: entities.CategoryExpense})

	tx := buildExpense(t, cash, bucket, 100_00, "UZS")
	require.NoError(t, repo.Create(ctx, &tx))

	require.NoError(t, repo.SetCategory(ctx, tx.ID, food.ID))

	got, err := repo.Get(ctx, tx.ID)
	require.NoError(t, err)
	require.NotNil(t, got.CategoryID)
	assert.Equal(t, food.ID, *got.CategoryID)

	// it no longer appears in the uncategorized bucket
	rest, err := repo.List(ctx, transactionrepository.Filter{Uncategorized: true, Limit: 100})
	require.NoError(t, err)
	assert.Empty(t, rest)
}

func TestCategory_MatchRule(t *testing.T) {
	reset(t)
	ctx := context.Background()
	repo := categoryRepo()

	food := mustCategory(t, entities.Category{Name: "Food", Type: entities.CategoryExpense})
	_, err := testDB.Pool.Exec(ctx,
		`INSERT INTO category_rules (pattern, category_id) VALUES ($1, $2)`, "HAVAS", food.ID)
	require.NoError(t, err)

	match, err := repo.MatchRule(ctx, entities.CategoryExpense, "SP OOO HAVAS FOOD>T")
	require.NoError(t, err)
	require.NotNil(t, match)
	assert.Equal(t, food.ID, *match)

	none, err := repo.MatchRule(ctx, entities.CategoryExpense, "PAYME OPLATA>TASHKEN")
	require.NoError(t, err)
	assert.Nil(t, none)

	// a block (category_rule with NULL category) must never route
	_, err = testDB.Pool.Exec(ctx,
		`INSERT INTO category_rules (pattern, category_id) VALUES ($1, NULL)`, "payme")
	require.NoError(t, err)
	blocked, err := repo.MatchRule(ctx, entities.CategoryExpense, "PAYME OPLATA>TASHKEN")
	require.NoError(t, err)
	assert.Nil(t, blocked, "a NULL-category block never routes")
}

func TestCategoryRule_Blocks(t *testing.T) {
	reset(t)
	ctx := context.Background()
	rules := categoryrulerepository.NewRepository(testDB)

	food := mustCategory(t, entities.Category{Name: "Food", Type: entities.CategoryExpense})
	require.NoError(t, rules.Create(ctx, &entities.CategoryRule{Pattern: "HAVAS", CategoryID: &food.ID}))

	// blocks are lower-cased and idempotent
	b1, err := rules.AddBlock(ctx, "PAYME OPLATA>TASHKEN")
	require.NoError(t, err)
	assert.Equal(t, "payme oplata>tashken", b1.Pattern)
	b2, err := rules.AddBlock(ctx, "payme oplata>tashken")
	require.NoError(t, err)
	assert.Equal(t, b1.ID, b2.ID, "re-blocking the same merchant is a no-op")

	blocked, err := rules.ListBlocked(ctx)
	require.NoError(t, err)
	require.Len(t, blocked, 1)

	// routing-rule listing excludes blocks
	routing, err := rules.List(ctx)
	require.NoError(t, err)
	require.Len(t, routing, 1)
	require.NotNil(t, routing[0].CategoryID)
	assert.Equal(t, food.ID, *routing[0].CategoryID)
}
