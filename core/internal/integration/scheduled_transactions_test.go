package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"finance/config"
	"finance/internal/entities"
	scheduledtransactionrepository "finance/internal/repositories/scheduled_transaction_repository"
	"finance/internal/scheduler"
	"finance/pkg/money"
)

func scheduleRepo() scheduledtransactionrepository.Repository {
	return scheduledtransactionrepository.NewRepository(testDB)
}

func materializer(base string) *scheduler.Materializer {
	return scheduler.NewMaterializer(accountRepo(), categoryRepo(), transactionRepo(), scheduleRepo(),
		&config.Finance{BaseCurrency: base}, zap.NewNop())
}

func TestScheduled_RunDueMaterializesAndAdvances(t *testing.T) {
	reset(t)
	ctx := context.Background()

	acc := mustAccount(t, assetAccount("USD", 0))
	cat := mustCategory(t, entities.Category{Name: "Rent", Type: entities.CategoryExpense})

	yesterday := time.Now().UTC().AddDate(0, 0, -1)
	sc := entities.ScheduledTransaction{
		Name:          ptr("Rent"),
		Type:          entities.TxExpense,
		FromAccountID: &acc.ID,
		CategoryID:    &cat.ID,
		Amount:        money.New(150000, "USD"),
		Frequency:     entities.FreqMonthly,
		Interval:      1,
		NextRun:       yesterday,
	}
	require.NoError(t, scheduleRepo().Create(ctx, &sc))

	now := time.Now().UTC()
	fired, err := materializer("USD").RunDue(ctx, now)
	require.NoError(t, err)
	assert.Equal(t, 1, fired)

	// exactly one transaction landed, with the template's account/category/amount
	txns, err := transactionRepo().List(ctx, allFilter())
	require.NoError(t, err)
	require.Len(t, txns, 1)
	assert.Equal(t, entities.TxExpense, txns[0].Type)
	assert.Equal(t, int64(150000), txns[0].Amount.Minor())
	assert.Equal(t, acc.ID, *txns[0].FromAccountID)
	assert.Equal(t, cat.ID, *txns[0].CategoryID)

	// next_run advanced into the future; last_run_at recorded
	got, err := scheduleRepo().Get(ctx, sc.ID)
	require.NoError(t, err)
	assert.True(t, got.NextRun.After(now), "next_run should be in the future")
	require.NotNil(t, got.LastRunAt)

	// a second tick at the same instant must not double-post (no longer due)
	fired, err = materializer("USD").RunDue(ctx, now)
	require.NoError(t, err)
	assert.Equal(t, 0, fired)
}

func TestScheduled_PausedAndExpiredAreSkipped(t *testing.T) {
	reset(t)
	ctx := context.Background()

	acc := mustAccount(t, assetAccount("USD", 0))
	cat := mustCategory(t, entities.Category{Name: "Gym", Type: entities.CategoryExpense})
	yesterday := time.Now().UTC().AddDate(0, 0, -1)

	paused := entities.ScheduledTransaction{
		Type: entities.TxExpense, FromAccountID: &acc.ID, CategoryID: &cat.ID, Amount: money.New(1000, "USD"),
		Frequency: entities.FreqMonthly, Interval: 1, NextRun: yesterday, Paused: true,
	}
	require.NoError(t, scheduleRepo().Create(ctx, &paused))

	lastWeek := time.Now().UTC().AddDate(0, 0, -7)
	expired := entities.ScheduledTransaction{
		Type: entities.TxExpense, FromAccountID: &acc.ID, CategoryID: &cat.ID, Amount: money.New(1000, "USD"),
		Frequency: entities.FreqMonthly, Interval: 1, NextRun: yesterday, EndDate: &lastWeek,
	}
	require.NoError(t, scheduleRepo().Create(ctx, &expired))

	fired, err := materializer("USD").RunDue(ctx, time.Now().UTC())
	require.NoError(t, err)
	assert.Equal(t, 0, fired)

	txns, err := transactionRepo().List(ctx, allFilter())
	require.NoError(t, err)
	assert.Empty(t, txns)
}
