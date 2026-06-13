package integration

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"finance/internal/entities"
	categoryrepository "finance/internal/repositories/category_repository"
)

func TestCategory_CRUD(t *testing.T) {
	reset(t)
	ctx := context.Background()
	repo := categoryRepo()

	food := mustCategory(t, entities.Category{Name: "Food", Type: entities.CategoryExpense})
	groceries := entities.Category{Name: "Groceries", Type: entities.CategoryExpense, ParentID: &food.ID}
	require.NoError(t, repo.Create(ctx, &groceries))
	mustCategory(t, entities.Category{Name: "Salary", Type: entities.CategoryIncome})

	expense, err := repo.List(ctx, ptr(entities.CategoryExpense), false)
	require.NoError(t, err)
	assert.Len(t, expense, 2)

	income, err := repo.List(ctx, ptr(entities.CategoryIncome), false)
	require.NoError(t, err)
	assert.Len(t, income, 1)

	// update
	groceries.Name = "Supermarket"
	groceries.Archived = true
	require.NoError(t, repo.Update(ctx, &groceries))
	got, err := repo.Get(ctx, groceries.ID)
	require.NoError(t, err)
	assert.Equal(t, "Supermarket", got.Name)
	assert.True(t, got.Archived)
}

func TestCategory_ParentTypeConstraint(t *testing.T) {
	reset(t)
	ctx := context.Background()
	repo := categoryRepo()

	expenseParent := mustCategory(t, entities.Category{Name: "Food", Type: entities.CategoryExpense})
	// income child under an expense parent violates the composite FK
	bad := entities.Category{Name: "Bad", Type: entities.CategoryIncome, ParentID: &expenseParent.ID}
	require.Error(t, repo.Create(ctx, &bad))
}

func TestCategory_DeleteWithChild(t *testing.T) {
	reset(t)
	ctx := context.Background()
	repo := categoryRepo()

	food := mustCategory(t, entities.Category{Name: "Food", Type: entities.CategoryExpense})
	groceries := entities.Category{Name: "Groceries", Type: entities.CategoryExpense, ParentID: &food.ID}
	require.NoError(t, repo.Create(ctx, &groceries))

	// parent with a child cannot be deleted
	assert.ErrorIs(t, repo.Delete(ctx, food.ID), categoryrepository.ErrInUse)
	// leaf can
	require.NoError(t, repo.Delete(ctx, groceries.ID))
	// now the parent can too
	require.NoError(t, repo.Delete(ctx, food.ID))
}

func TestCategory_DeleteInUseByTransaction(t *testing.T) {
	reset(t)
	ctx := context.Background()

	cash := mustAccount(t, assetAccount("UZS", 0))
	food := mustCategory(t, entities.Category{Name: "Food", Type: entities.CategoryExpense})
	tx := buildExpense(t, cash, food, 100, "UZS")
	require.NoError(t, transactionRepo().Create(ctx, &tx))

	assert.ErrorIs(t, categoryRepo().Delete(ctx, food.ID), categoryrepository.ErrInUse)
}

func TestCategory_NotFound(t *testing.T) {
	reset(t)
	_, err := categoryRepo().Get(context.Background(), uuid.New())
	assert.ErrorIs(t, err, categoryrepository.ErrNotFound)
}
