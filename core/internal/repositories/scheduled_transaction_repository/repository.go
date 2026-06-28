package scheduledtransactionrepository

import (
	"context"
	"errors"
	"fmt"

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
}

type repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) Repository {
	return &repository{db: db}
}

// rate_to_base is cast to text so it scans into fx.Rate without relying on pgx
// numeric decoding (mirrors transaction_repository). amount/to_amount are
// money_t composites.
const columns = `id, name, type, from_account_id, to_account_id, category_id,
	amount, to_amount, rate_to_base::text AS rate_to_base, note, tags,
	frequency, interval, start_date, end_date, paused, created_at`

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
			 rate_to_base, note, tags, frequency, interval, start_date, end_date, paused)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8::numeric, $9, $10, $11, $12, $13, $14, $15)
		RETURNING id, created_at`

	err := r.db.Pool.QueryRow(ctx, query,
		s.Name, s.Type, s.FromAccountID, s.ToAccountID, s.CategoryID, s.Amount, s.ToAmount,
		rateText(s.RateToBase), s.Note, tags(s.Tags), s.Frequency, s.Interval, s.StartDate, s.EndDate, s.Paused,
	).Scan(&s.ID, &s.CreatedAt)
	if err != nil {
		return fmt.Errorf("create scheduled transaction: %w", err)
	}

	return nil
}

func (r repository) List(ctx context.Context) ([]entities.ScheduledTransaction, error) {
	return r.query(ctx, `SELECT `+columns+` FROM scheduled_transactions ORDER BY created_at`)
}

func (r repository) query(ctx context.Context, query string, args ...any) ([]entities.ScheduledTransaction, error) {
	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list scheduled transactions: %w", err)
	}

	out, err := pgx.CollectRows(rows, pgx.RowToStructByName[entities.ScheduledTransaction])
	if err != nil {
		return nil, fmt.Errorf("list scheduled transactions: %w", err)
	}

	return out, nil
}

func (r repository) Get(ctx context.Context, id uuid.UUID) (*entities.ScheduledTransaction, error) {
	rows, err := r.db.Pool.Query(ctx, `SELECT `+columns+` FROM scheduled_transactions WHERE id = $1`, id)
	if err != nil {
		return nil, fmt.Errorf("get scheduled transaction: %w", err)
	}

	s, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[entities.ScheduledTransaction])
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
			frequency = $12, interval = $13, start_date = $14, end_date = $15, paused = $16
		WHERE id = $1`

	res, err := r.db.Pool.Exec(ctx, query,
		s.ID, s.Name, s.Type, s.FromAccountID, s.ToAccountID, s.CategoryID,
		s.Amount, s.ToAmount, rateText(s.RateToBase), s.Note, tags(s.Tags),
		s.Frequency, s.Interval, s.StartDate, s.EndDate, s.Paused,
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
