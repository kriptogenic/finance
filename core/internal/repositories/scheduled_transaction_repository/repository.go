package scheduledtransactionrepository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"finance/internal/entities"
	"finance/pkg/database"
	"finance/pkg/fx"
)

var ErrNotFound = errors.New("scheduled transaction not found")

type Repository interface {
	Create(ctx context.Context, s *entities.ScheduledTransaction) error
	List(ctx context.Context) ([]entities.ScheduledTransaction, error)
	Get(ctx context.Context, id uuid.UUID) (*entities.ScheduledTransaction, error)
	Update(ctx context.Context, s *entities.ScheduledTransaction) error
	Delete(ctx context.Context, id uuid.UUID) error
	// Due returns active schedules whose next_run has arrived by asOf.
	Due(ctx context.Context, asOf time.Time) ([]entities.ScheduledTransaction, error)
	// Advance persists the post-run state (next_run + last_run_at).
	Advance(ctx context.Context, id uuid.UUID, nextRun, lastRunAt time.Time) error
}

type repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) Repository {
	return &repository{db: db}
}

// rate_to_base is cast to text so it parses into fx.Rate without relying on pgx
// numeric decoding (mirrors transaction_repository).
const columns = `id, name, type, from_account_id, to_account_id, category_id,
	amount, to_amount, rate_to_base::text, note, tags,
	frequency, interval, next_run, end_date, paused, last_run_at, created_at`

func scanScheduled(row pgx.Row) (entities.ScheduledTransaction, error) {
	var (
		s        entities.ScheduledTransaction
		rateText *string
	)
	err := row.Scan(
		&s.ID, &s.Name, &s.Type, &s.FromAccountID, &s.ToAccountID, &s.CategoryID,
		&s.Amount, &s.ToAmount, &rateText, &s.Note, &s.Tags,
		&s.Frequency, &s.Interval, &s.NextRun, &s.EndDate, &s.Paused, &s.LastRunAt, &s.CreatedAt,
	)
	if err != nil {
		return entities.ScheduledTransaction{}, err
	}

	if rateText != nil {
		rate, parseErr := fx.ParseRate(*rateText)
		if parseErr != nil {
			return entities.ScheduledTransaction{}, fmt.Errorf("parse rate_to_base: %w", parseErr)
		}
		s.RateToBase = &rate
	}

	return s, nil
}

func rateText(rate *fx.Rate) *string {
	if rate == nil {
		return nil
	}
	str := rate.String()

	return &str
}

// tags coalesces a nil slice to an empty one so the NOT NULL tags column accepts it.
func tags(t []string) []string {
	if t == nil {
		return []string{}
	}

	return t
}

func (r repository) Create(ctx context.Context, s *entities.ScheduledTransaction) error {
	const query = `
		INSERT INTO scheduled_transactions
			(name, type, from_account_id, to_account_id, category_id, amount, to_amount,
			 rate_to_base, note, tags, frequency, interval, next_run, end_date, paused)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8::numeric, $9, $10, $11, $12, $13, $14, $15)
		RETURNING id, created_at`

	err := r.db.Pool.QueryRow(ctx, query,
		s.Name, s.Type, s.FromAccountID, s.ToAccountID, s.CategoryID, s.Amount, s.ToAmount,
		rateText(s.RateToBase), s.Note, tags(s.Tags), s.Frequency, s.Interval, s.NextRun, s.EndDate, s.Paused,
	).Scan(&s.ID, &s.CreatedAt)
	if err != nil {
		return fmt.Errorf("create scheduled transaction: %w", err)
	}

	return nil
}

func (r repository) List(ctx context.Context) ([]entities.ScheduledTransaction, error) {
	return r.query(ctx, `SELECT `+columns+` FROM scheduled_transactions ORDER BY created_at`)
}

func (r repository) Due(ctx context.Context, asOf time.Time) ([]entities.ScheduledTransaction, error) {
	const query = `SELECT ` + columns + ` FROM scheduled_transactions
		WHERE paused = FALSE AND next_run <= $1 AND (end_date IS NULL OR next_run <= end_date)
		ORDER BY next_run`

	return r.query(ctx, query, asOf)
}

func (r repository) query(ctx context.Context, query string, args ...any) ([]entities.ScheduledTransaction, error) {
	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list scheduled transactions: %w", err)
	}
	defer rows.Close()

	var out []entities.ScheduledTransaction
	for rows.Next() {
		s, scanErr := scanScheduled(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("list scheduled transactions: %w", scanErr)
		}
		out = append(out, s)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("list scheduled transactions: %w", err)
	}

	return out, nil
}

func (r repository) Get(ctx context.Context, id uuid.UUID) (*entities.ScheduledTransaction, error) {
	s, err := scanScheduled(r.db.Pool.QueryRow(ctx, `SELECT `+columns+` FROM scheduled_transactions WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get scheduled transaction: %w", err)
	}

	return &s, nil
}

func (r repository) Update(ctx context.Context, s *entities.ScheduledTransaction) error {
	const query = `
		UPDATE scheduled_transactions SET
			name = $2, type = $3, from_account_id = $4, to_account_id = $5, category_id = $6,
			amount = $7, to_amount = $8, rate_to_base = $9::numeric, note = $10, tags = $11,
			frequency = $12, interval = $13, next_run = $14, end_date = $15, paused = $16
		WHERE id = $1`

	res, err := r.db.Pool.Exec(ctx, query,
		s.ID, s.Name, s.Type, s.FromAccountID, s.ToAccountID, s.CategoryID,
		s.Amount, s.ToAmount, rateText(s.RateToBase), s.Note, tags(s.Tags),
		s.Frequency, s.Interval, s.NextRun, s.EndDate, s.Paused,
	)
	if err != nil {
		return fmt.Errorf("update scheduled transaction: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

func (r repository) Delete(ctx context.Context, id uuid.UUID) error {
	res, err := r.db.Pool.Exec(ctx, `DELETE FROM scheduled_transactions WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete scheduled transaction: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

func (r repository) Advance(ctx context.Context, id uuid.UUID, nextRun, lastRunAt time.Time) error {
	res, err := r.db.Pool.Exec(ctx,
		`UPDATE scheduled_transactions SET next_run = $2, last_run_at = $3 WHERE id = $1`,
		id, nextRun, lastRunAt)
	if err != nil {
		return fmt.Errorf("advance scheduled transaction: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}
