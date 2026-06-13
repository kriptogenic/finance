package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"finance/internal/entities"
	budgetrepository "finance/internal/repositories/budget_repository"
)

func budgetRepo() budgetrepository.Repository { return budgetrepository.NewRepository(testDB) }

func TestBudget_CRUD(t *testing.T) {
	reset(t)
	ctx := context.Background()
	repo := budgetRepo()

	food := mustCategory(t, entities.Category{Name: "Food", Type: entities.CategoryExpense})

	b := entities.Budget{CategoryID: food.ID, Period: entities.BudgetMonthly, Amount: 1000000}
	require.NoError(t, repo.Create(ctx, &b))

	got, err := repo.Get(ctx, b.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(1000000), got.Amount)
	assert.Equal(t, entities.BudgetMonthly, got.Period)

	got.Amount = 1500000
	got.Period = entities.BudgetWeekly
	require.NoError(t, repo.Update(ctx, got))
	got, _ = repo.Get(ctx, b.ID)
	assert.Equal(t, int64(1500000), got.Amount)
	assert.Equal(t, entities.BudgetWeekly, got.Period)

	require.NoError(t, repo.Delete(ctx, b.ID))
	_, err = repo.Get(ctx, b.ID)
	assert.ErrorIs(t, err, budgetrepository.ErrNotFound)
}

func TestBudget_UniqueAndCascade(t *testing.T) {
	reset(t)
	ctx := context.Background()
	repo := budgetRepo()

	food := mustCategory(t, entities.Category{Name: "Food", Type: entities.CategoryExpense})
	b1 := entities.Budget{CategoryID: food.ID, Period: entities.BudgetMonthly, Amount: 1000}
	require.NoError(t, repo.Create(ctx, &b1))

	// one budget per category
	b2 := entities.Budget{CategoryID: food.ID, Period: entities.BudgetMonthly, Amount: 2000}
	require.Error(t, repo.Create(ctx, &b2))

	// deleting the category cascades to its budget
	require.NoError(t, categoryRepo().Delete(ctx, food.ID))
	_, err := repo.Get(ctx, b1.ID)
	assert.ErrorIs(t, err, budgetrepository.ErrNotFound)
}

func TestBudget_SpentCoversSubcategories(t *testing.T) {
	reset(t)
	ctx := context.Background()
	repo := budgetRepo()

	cash := mustAccount(t, assetAccount("UZS", 0))
	food := mustCategory(t, entities.Category{Name: "Food", Type: entities.CategoryExpense})
	groceries := entities.Category{Name: "Groceries", Type: entities.CategoryExpense, ParentID: &food.ID}
	require.NoError(t, categoryRepo().Create(ctx, &groceries))
	rent := mustCategory(t, entities.Category{Name: "Rent", Type: entities.CategoryExpense})

	now := time.Now().UTC()
	inWindow := time.Date(now.Year(), now.Month(), 5, 12, 0, 0, 0, time.UTC)

	add := func(cat entities.Category, amount int64) {
		tx := buildExpense(t, cash, cat, amount, "UZS")
		tx.Date = inWindow
		require.NoError(t, transactionRepo().Create(ctx, &tx))
	}
	add(food, 200000)      // directly on Food
	add(groceries, 300000) // subcategory rolls up
	add(rent, 999999)      // different category, excluded

	// previous-month expense is outside the window
	last := inWindow.AddDate(0, -1, 0)
	prev := buildExpense(t, cash, food, 700000, "UZS")
	prev.Date = last
	require.NoError(t, transactionRepo().Create(ctx, &prev))

	start, end := entities.PeriodWindow(entities.BudgetMonthly, now)
	spent, err := repo.Spent(ctx, food.ID, start, end)
	require.NoError(t, err)
	assert.Equal(t, int64(500000), spent, "Food + Groceries within the month, excluding Rent and last month")
}
