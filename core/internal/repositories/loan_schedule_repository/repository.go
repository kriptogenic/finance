package loanschedulerepository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"finance/internal/entities"
	"finance/pkg/database"
)

var ErrNotFound = errors.New("loan schedule row not found")

type Repository interface {
	// ListByAccount returns a loan's installments ordered by period.
	ListByAccount(ctx context.Context, accountID uuid.UUID) ([]entities.LoanSchedule, error)
	GetRow(ctx context.Context, id uuid.UUID) (*entities.LoanSchedule, error)
	// Replace atomically swaps a loan's whole schedule for the given rows.
	Replace(ctx context.Context, accountID uuid.UUID, rows []entities.LoanSchedule) error
	// UpdateRow persists the editable fields of a single installment.
	UpdateRow(ctx context.Context, row *entities.LoanSchedule) error
}

type repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) Repository {
	return &repository{db: db}
}

const columns = `id, account_id, period, due_date, date_override,
	payment, principal, interest, balance, paid, created_at`

func (r repository) ListByAccount(ctx context.Context, accountID uuid.UUID) ([]entities.LoanSchedule, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT `+columns+` FROM loan_schedules WHERE account_id = $1 ORDER BY period`, accountID)
	if err != nil {
		return nil, fmt.Errorf("list loan schedule: %w", err)
	}

	sched, err := pgx.CollectRows(rows, pgx.RowToStructByName[entities.LoanSchedule])
	if err != nil {
		return nil, fmt.Errorf("list loan schedule: %w", err)
	}

	return sched, nil
}

func (r repository) GetRow(ctx context.Context, id uuid.UUID) (*entities.LoanSchedule, error) {
	rows, err := r.db.Pool.Query(ctx, `SELECT `+columns+` FROM loan_schedules WHERE id = $1`, id)
	if err != nil {
		return nil, fmt.Errorf("get loan schedule row: %w", err)
	}

	row, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[entities.LoanSchedule])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get loan schedule row: %w", err)
	}

	return &row, nil
}

func (r repository) Replace(ctx context.Context, accountID uuid.UUID, rows []entities.LoanSchedule) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("replace loan schedule: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit

	if _, err = tx.Exec(ctx, `DELETE FROM loan_schedules WHERE account_id = $1`, accountID); err != nil {
		return fmt.Errorf("replace loan schedule: %w", err)
	}

	for i := range rows {
		row := &rows[i]
		_, err = tx.Exec(ctx, `
			INSERT INTO loan_schedules
				(account_id, period, due_date, date_override, payment, principal, interest, balance, paid)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			accountID, row.Period, row.DueDate, row.DateOverride,
			row.Payment, row.Principal, row.Interest, row.Balance, row.Paid)
		if err != nil {
			return fmt.Errorf("replace loan schedule: %w", err)
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("replace loan schedule: %w", err)
	}

	return nil
}

func (r repository) UpdateRow(ctx context.Context, row *entities.LoanSchedule) error {
	res, err := r.db.Pool.Exec(ctx, `
		UPDATE loan_schedules SET
			due_date = $2, date_override = $3, payment = $4,
			principal = $5, interest = $6, balance = $7, paid = $8
		WHERE id = $1`,
		row.ID, row.DueDate, row.DateOverride,
		row.Payment, row.Principal, row.Interest, row.Balance, row.Paid)
	if err != nil {
		return fmt.Errorf("update loan schedule row: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}
