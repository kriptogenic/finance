package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"finance/internal/entities"
	"finance/internal/ledger"
	holidayrepository "finance/internal/repositories/holiday_repository"
	loanschedulerepository "finance/internal/repositories/loan_schedule_repository"
	"finance/pkg/money"
)

func loanScheduleRepo() loanschedulerepository.Repository {
	return loanschedulerepository.NewRepository(testDB)
}

func holidayRepo() holidayrepository.Repository {
	return holidayrepository.NewRepository(testDB)
}

func loanAccount(t *testing.T) entities.Account {
	t.Helper()

	return mustAccount(t, entities.Account{
		Name: "Home Loan", Kind: entities.KindLiability, Type: entities.TypeLoan, Currency: "UZS",
		OpeningBalance: money.New(60_000_000_00, "UZS"), Principal: ptr(money.New(60_000_000_00, "UZS")),
		InterestRate: ptr(0.16), TermMonths: ptr(12), PaymentDay: ptr(10),
		StartDate: ptr(time.Date(2026, time.January, 10, 0, 0, 0, 0, time.UTC)),
	})
}

func genRows(t *testing.T, acc entities.Account, cal ledger.Calendar, overrides map[int]time.Time) []entities.LoanSchedule {
	t.Helper()
	sched, err := ledger.GenerateSchedule(ledger.ScheduleParams{
		Principal: *acc.Principal, AnnualRate: *acc.InterestRate, TermMonths: *acc.TermMonths,
		Start: *acc.StartDate, PaymentDay: *acc.PaymentDay, Calendar: cal, Overrides: overrides,
	})
	require.NoError(t, err)

	rows := make([]entities.LoanSchedule, len(sched.Rows))
	for i, r := range sched.Rows {
		rows[i] = entities.LoanSchedule{
			AccountID: acc.ID, Period: r.Period, DueDate: r.Date,
			Payment: r.Payment, Principal: r.Principal, Interest: r.Interest, Balance: r.Balance,
		}
		if ov, ok := overrides[r.Period]; ok {
			rows[i].DateOverride = &ov
		}
	}

	return rows
}

func TestLoanSchedule_ReplaceAndList(t *testing.T) {
	reset(t)
	ctx := context.Background()
	acc := loanAccount(t)
	repo := loanScheduleRepo()

	require.NoError(t, repo.Replace(ctx, acc.ID, genRows(t, acc, nil, nil)))

	rows, err := repo.ListByAccount(ctx, acc.ID)
	require.NoError(t, err)
	require.Len(t, rows, 12)

	var sumPrincipal int64
	for _, r := range rows {
		assert.Equal(t, r.Principal.Minor()+r.Interest.Minor(), r.Payment.Minor(), "period %d reconciles", r.Period)
		sumPrincipal += r.Principal.Minor()
	}
	assert.Equal(t, int64(0), rows[11].Balance.Minor(), "loan fully repaid")
	assert.Equal(t, int64(60_000_000_00), sumPrincipal)

	// Replace is a full swap, not an append
	require.NoError(t, repo.Replace(ctx, acc.ID, genRows(t, acc, nil, nil)))
	rows, err = repo.ListByAccount(ctx, acc.ID)
	require.NoError(t, err)
	assert.Len(t, rows, 12)
}

func TestLoanSchedule_HolidayRollAndOverride(t *testing.T) {
	reset(t)
	ctx := context.Background()
	acc := loanAccount(t)

	// seed a holiday, load it into the ledger calendar
	_, err := testDB.Pool.Exec(ctx,
		`INSERT INTO holidays (day, name) VALUES ($1, $2) ON CONFLICT (day) DO NOTHING`,
		time.Date(2026, time.March, 10, 0, 0, 0, 0, time.UTC), "Test Holiday")
	require.NoError(t, err)

	holidays, err := holidayRepo().List(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, holidays)
	days := make([]time.Time, len(holidays))
	for i, h := range holidays {
		days[i] = h.Day
	}
	cal := ledger.NewHolidaySet(days)

	forced := time.Date(2026, time.February, 12, 0, 0, 0, 0, time.UTC)
	rows := genRows(t, acc, cal, map[int]time.Time{2: forced})
	require.NoError(t, loanScheduleRepo().Replace(ctx, acc.ID, rows))

	stored, err := loanScheduleRepo().ListByAccount(ctx, acc.ID)
	require.NoError(t, err)

	// no installment lands on the holiday or a weekend
	for _, r := range stored {
		assert.NotEqual(t, time.Saturday, r.DueDate.Weekday())
		assert.NotEqual(t, time.Sunday, r.DueDate.Weekday())
		assert.False(t, cal.IsHoliday(r.DueDate), "period %d off holidays", r.Period)
	}
	// the override round-trips
	require.NotNil(t, stored[1].DateOverride)
	assert.Equal(t, forced, stored[1].DateOverride.UTC())
	assert.Equal(t, forced, stored[1].DueDate.UTC())
}

func TestLoanSchedule_UpdateRowMarksPaid(t *testing.T) {
	reset(t)
	ctx := context.Background()
	acc := loanAccount(t)
	repo := loanScheduleRepo()
	require.NoError(t, repo.Replace(ctx, acc.ID, genRows(t, acc, nil, nil)))

	rows, err := repo.ListByAccount(ctx, acc.ID)
	require.NoError(t, err)

	row := rows[0]
	row.Paid = true
	require.NoError(t, repo.UpdateRow(ctx, &row))

	got, err := repo.GetRow(ctx, row.ID)
	require.NoError(t, err)
	assert.True(t, got.Paid)
}

// The composite CHECK rejects a row whose payment != principal + interest.
func TestLoanSchedule_SplitConstraint(t *testing.T) {
	reset(t)
	ctx := context.Background()
	acc := loanAccount(t)

	bad := []entities.LoanSchedule{{
		AccountID: acc.ID, Period: 1, DueDate: *acc.StartDate,
		Payment:   money.New(1000, "UZS"),
		Principal: money.New(400, "UZS"),
		Interest:  money.New(500, "UZS"), // 400 + 500 != 1000
		Balance:   money.New(0, "UZS"),
	}}
	err := loanScheduleRepo().Replace(ctx, acc.ID, bad)
	require.Error(t, err)
}
