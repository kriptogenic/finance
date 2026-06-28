package budgetrepository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"finance/config"
	"finance/internal/entities"
	"finance/pkg/database"
	"finance/pkg/money"
)

var ErrNotFound = errors.New("budget not found")

type Repository interface {
	Create(ctx context.Context, b *entities.Budget) error
	List(ctx context.Context) ([]entities.Budget, error)
	Get(ctx context.Context, id uuid.UUID) (*entities.Budget, error)
	Update(ctx context.Context, b *entities.Budget) error
	Delete(ctx context.Context, id uuid.UUID) error
	// Spent sums expense base_amount for the category and its subcategories
	// within [from, to). Income and transfers are excluded (App. A).
	Spent(ctx context.Context, categoryID uuid.UUID, from, to time.Time) (money.Money, error)
}

type repository struct {
	db   *database.DB
	base string
}

func NewRepository(db *database.DB, finance *config.Finance) Repository {
	return &repository{db: db, base: finance.BaseCurrency}
}

const columns = `id, category_id, period, amount, rollover, start_period, created_at`

func (r repository) Create(ctx context.Context, b *entities.Budget) error {
	const query = `
		INSERT INTO budgets (category_id, period, amount, rollover, start_period)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at`

	err := r.db.Pool.QueryRow(ctx, query, b.CategoryID, b.Period, b.Amount, b.Rollover, b.StartPeriod).
		Scan(&b.ID, &b.CreatedAt)
	if err != nil {
		return fmt.Errorf("create budget: %w", err)
	}

	return nil
}

func (r repository) List(ctx context.Context) ([]entities.Budget, error) {
	rows, err := r.db.Pool.Query(ctx, `SELECT `+columns+` FROM budgets ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("list budgets: %w", err)
	}

	budgets, err := pgx.CollectRows(rows, pgx.RowToStructByName[entities.Budget])
	if err != nil {
		return nil, fmt.Errorf("list budgets: %w", err)
	}

	return budgets, nil
}

func (r repository) Get(ctx context.Context, id uuid.UUID) (*entities.Budget, error) {
	rows, err := r.db.Pool.Query(ctx, `SELECT `+columns+` FROM budgets WHERE id = $1`, id)
	if err != nil {
		return nil, fmt.Errorf("get budget: %w", err)
	}

	b, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[entities.Budget])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get budget: %w", err)
	}

	return &b, nil
}

func (r repository) Update(ctx context.Context, b *entities.Budget) error {
	const query = `
		UPDATE budgets SET period = $2, amount = $3, rollover = $4, start_period = $5
		WHERE id = $1`

	res, err := r.db.Pool.Exec(ctx, query, b.ID, b.Period, b.Amount, b.Rollover, b.StartPeriod)
	if err != nil {
		return fmt.Errorf("update budget: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

func (r repository) Delete(ctx context.Context, id uuid.UUID) error {
	res, err := r.db.Pool.Exec(ctx, `DELETE FROM budgets WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete budget: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

func (r repository) Spent(ctx context.Context, categoryID uuid.UUID, from, to time.Time) (money.Money, error) {
	const query = `
		SELECT COALESCE(SUM(COALESCE((t.base_amount).amount, (t.amount).amount)), 0)::bigint
		FROM transactions t
		JOIN categories c ON c.id = t.category_id
		WHERE t.type = 'expense'
		  AND (c.id = $1 OR c.parent_id = $1)
		  AND t.date >= $2 AND t.date < $3`

	var spent int64
	if err := r.db.Pool.QueryRow(ctx, query, categoryID, from, to).Scan(&spent); err != nil {
		return money.Money{}, fmt.Errorf("budget spent: %w", err)
	}

	return money.New(spent, r.base), nil
}
