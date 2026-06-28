package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"finance/internal/entities"
	"finance/internal/ledger"
	scheduledtransactionrepository "finance/internal/repositories/scheduled_transaction_repository"
	"finance/pkg/fx"
	"finance/pkg/money"
)

func scheduleRepo() scheduledtransactionrepository.Repository {
	return scheduledtransactionrepository.NewRepository(testDB)
}

func TestScheduled_CRUDRoundTrip(t *testing.T) {
	reset(t)
	ctx := context.Background()

	acc := mustAccount(t, assetAccount("USD", 0))
	cat := mustCategory(t, entities.Category{Name: "Rent", Type: entities.CategoryExpense})

	start := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)
	sc := entities.ScheduledTransaction{
		Name:          ptr("Rent"),
		Type:          entities.TxExpense,
		FromAccountID: &acc.ID,
		CategoryID:    &cat.ID,
		Amount:        money.New(150000, "USD"),
		Frequency:     entities.FreqMonthly,
		Interval:      1,
		StartDate:     start,
	}
	require.NoError(t, scheduleRepo().Create(ctx, &sc))

	got, err := scheduleRepo().Get(ctx, sc.ID)
	require.NoError(t, err)
	assert.Equal(t, "Rent", *got.Name)
	assert.Equal(t, "2026-06-01", got.StartDate.Format("2006-01-02"))

	got.Amount = money.New(200000, "USD")
	require.NoError(t, scheduleRepo().Update(ctx, got))
	after, err := scheduleRepo().Get(ctx, sc.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(200000), after.Amount.Minor())

	require.NoError(t, scheduleRepo().Delete(ctx, sc.ID))
	_, err = scheduleRepo().Get(ctx, sc.ID)
	require.ErrorIs(t, err, scheduledtransactionrepository.ErrNotFound)
}

// A monthly salary in, rent out, and a card top-up transfer project into one
// month: expense combines the rent and the transfer outflow; net = income − expense.
func TestScheduled_ForecastMonth(t *testing.T) {
	reset(t)
	ctx := context.Background()

	checking := mustAccount(t, assetAccount("USD", 0))
	card := mustAccount(t, liabilityAccount("USD", 0))
	salaryCat := mustCategory(t, entities.Category{Name: "Salary", Type: entities.CategoryIncome})
	rentCat := mustCategory(t, entities.Category{Name: "Rent", Type: entities.CategoryExpense})

	start := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)
	repo := scheduleRepo()

	salary := entities.ScheduledTransaction{
		Type: entities.TxIncome, ToAccountID: &checking.ID, CategoryID: &salaryCat.ID,
		Amount: money.New(500000, "USD"), Frequency: entities.FreqMonthly, Interval: 1, StartDate: start,
	}
	rent := entities.ScheduledTransaction{
		Type: entities.TxExpense, FromAccountID: &checking.ID, CategoryID: &rentCat.ID,
		Amount: money.New(150000, "USD"), Frequency: entities.FreqMonthly, Interval: 1, StartDate: start,
	}
	topUp := entities.ScheduledTransaction{
		Type: entities.TxTransfer, FromAccountID: &checking.ID, ToAccountID: &card.ID,
		Amount: money.New(100000, "USD"), Frequency: entities.FreqMonthly, Interval: 1, StartDate: start,
	}
	require.NoError(t, repo.Create(ctx, &salary))
	require.NoError(t, repo.Create(ctx, &rent))
	require.NoError(t, repo.Create(ctx, &topUp))

	schedules, err := repo.List(ctx)
	require.NoError(t, err)

	monthStart := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)
	f := ledger.ForecastMonth("USD", schedules, nil, nil, monthStart, monthStart.AddDate(0, 1, 0), map[string]fx.Rate{})

	assert.Equal(t, int64(500000), f.Income.Minor())
	assert.Equal(t, int64(250000), f.Expense.Minor()) // 150000 rent + 100000 transfer
	assert.Equal(t, int64(250000), f.Net.Minor())
	assert.Len(t, f.Lines, 3)
	assert.Empty(t, f.MissingRates)
}
