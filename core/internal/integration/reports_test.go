package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"finance/internal/entities"
	"finance/internal/ledger"
	reportrepository "finance/internal/repositories/report_repository"
	"finance/pkg/fx"
)

func reportRepo() reportrepository.Repository { return reportrepository.NewRepository(testDB) }

func TestReport_LatestRates(t *testing.T) {
	reset(t)
	ctx := context.Background()
	repo := transactionRepo()

	usd := mustAccount(t, assetAccount("USD", 0))
	food := mustCategory(t, entities.Category{Name: "Food", Type: entities.CategoryExpense})

	mk := func(rate string, day int) {
		r := fx.MustParseRate(rate)
		tx := mustBuild(t, ledger.NewTransaction{
			Type: entities.TxExpense, From: &usd, Category: &food, Amount: 100, RateToBase: &r,
		}, "UZS")
		tx.Date = time.Date(2026, time.June, day, 12, 0, 0, 0, time.UTC)
		require.NoError(t, repo.Create(ctx, &tx))
	}
	mk("12000", 1)
	mk("12800", 20) // latest
	mk("12500", 10)

	rates, err := reportRepo().LatestRates(ctx)
	require.NoError(t, err)
	require.Contains(t, rates, "USD")
	assert.Equal(t, "12800.0000000000", rates["USD"].String())
}

func TestReport_SpendingByCategory(t *testing.T) {
	reset(t)
	ctx := context.Background()
	repo := transactionRepo()

	cash := mustAccount(t, assetAccount("UZS", 0))
	usd := mustAccount(t, assetAccount("USD", 0))
	food := mustCategory(t, entities.Category{Name: "Food", Type: entities.CategoryExpense})
	groceries := entities.Category{Name: "Groceries", Type: entities.CategoryExpense, ParentID: &food.ID}
	require.NoError(t, categoryRepo().Create(ctx, &groceries))
	rent := mustCategory(t, entities.Category{Name: "Rent", Type: entities.CategoryExpense})

	// 200000 directly on Food, 300000 on its subcategory -> rolls up to Food
	e1 := buildExpense(t, cash, food, 200000, "UZS")
	require.NoError(t, repo.Create(ctx, &e1))
	e2 := buildExpense(t, cash, groceries, 300000, "UZS")
	require.NoError(t, repo.Create(ctx, &e2))
	// 1.5M on Rent
	e3 := buildExpense(t, cash, rent, 1500000, "UZS")
	require.NoError(t, repo.Create(ctx, &e3))
	// USD expense 10.00 @ 12500 -> 125000 base, under Food
	rate := fx.MustParseRate("12500")
	e4 := mustBuild(t, ledger.NewTransaction{Type: entities.TxExpense, From: &usd, Category: &food, Amount: 1000, RateToBase: &rate}, "UZS")
	require.NoError(t, repo.Create(ctx, &e4))

	from := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	spend, err := reportRepo().SpendingByCategory(ctx, from, time.Now())
	require.NoError(t, err)

	byName := map[string]int64{}
	for _, s := range spend {
		byName[s.CategoryName] = s.Amount
	}
	// Food = 200000 + 300000 (subcat) + 12,500,000 (USD base) = 13,000,000
	assert.Equal(t, int64(13000000), byName["Food"])
	assert.Equal(t, int64(1500000), byName["Rent"])
}

func TestReport_CashFlow(t *testing.T) {
	reset(t)
	ctx := context.Background()
	repo := transactionRepo()

	cash := mustAccount(t, assetAccount("UZS", 0))
	food := mustCategory(t, entities.Category{Name: "Food", Type: entities.CategoryExpense})
	salary := mustCategory(t, entities.Category{Name: "Salary", Type: entities.CategoryIncome})

	may := time.Date(2026, time.May, 3, 12, 0, 0, 0, time.UTC)
	jun := time.Date(2026, time.June, 5, 12, 0, 0, 0, time.UTC)

	add := func(tx entities.Transaction, when time.Time) {
		tx.Date = when
		require.NoError(t, repo.Create(ctx, &tx))
	}
	add(buildIncome(t, cash, salary, 5000000, "UZS"), may)
	add(buildExpense(t, cash, food, 1800000, "UZS"), may)
	add(buildIncome(t, cash, salary, 5000000, "UZS"), jun)
	add(buildExpense(t, cash, food, 200000, "UZS"), jun)

	from := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	flow, err := reportRepo().CashFlow(ctx, from, time.Now())
	require.NoError(t, err)
	require.Len(t, flow, 2)

	byMonth := map[string]reportrepository.MonthFlow{}
	for _, m := range flow {
		byMonth[m.Month] = m
	}
	assert.Equal(t, int64(5000000), byMonth["2026-05"].Income)
	assert.Equal(t, int64(1800000), byMonth["2026-05"].Expense)
	assert.Equal(t, int64(5000000), byMonth["2026-06"].Income)
	assert.Equal(t, int64(200000), byMonth["2026-06"].Expense)
}
