package transactionrepository

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

var ErrNotFound = errors.New("transaction not found")

// Filter narrows a transaction search (REQUIREMENTS §6).
type Filter struct {
	AccountID  *uuid.UUID
	CategoryID *uuid.UUID
	Type       *entities.TransactionType
	DateFrom   *time.Time
	DateTo     *time.Time
	Tag        *string
	Query      *string
	Limit      int
	Offset     int
}

type Repository interface {
	Create(ctx context.Context, tx *entities.Transaction) error
	List(ctx context.Context, filter Filter) ([]entities.Transaction, error)
	Get(ctx context.Context, id uuid.UUID) (*entities.Transaction, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) Repository {
	return &repository{db: db}
}

// rate_to_base is cast to text so it can be parsed into fx.Rate without relying
// on pgx numeric decoding.
const txColumns = `id, date, type, from_account_id, to_account_id, category_id,
	amount, currency, to_amount, to_currency, rate_to_base::text, base_amount,
	note, tags, created_at`

func scanTransaction(row pgx.Row) (entities.Transaction, error) {
	var (
		t        entities.Transaction
		rateText *string
	)
	err := row.Scan(
		&t.ID, &t.Date, &t.Type, &t.FromAccountID, &t.ToAccountID, &t.CategoryID,
		&t.Amount, &t.Currency, &t.ToAmount, &t.ToCurrency, &rateText, &t.BaseAmount,
		&t.Note, &t.Tags, &t.CreatedAt,
	)
	if err != nil {
		return entities.Transaction{}, err
	}

	if rateText != nil {
		rate, parseErr := fx.ParseRate(*rateText)
		if parseErr != nil {
			return entities.Transaction{}, fmt.Errorf("parse rate_to_base: %w", parseErr)
		}
		t.RateToBase = &rate
	}

	return t, nil
}

func (r repository) Create(ctx context.Context, tx *entities.Transaction) error {
	var rateText *string
	if tx.RateToBase != nil {
		s := tx.RateToBase.String()
		rateText = &s
	}

	const query = `
		INSERT INTO transactions
			(date, type, from_account_id, to_account_id, category_id, amount, currency,
			 to_amount, to_currency, rate_to_base, base_amount, note, tags)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::numeric, $11, $12, $13)
		RETURNING id, created_at`

	err := r.db.Pool.QueryRow(ctx, query,
		tx.Date, tx.Type, tx.FromAccountID, tx.ToAccountID, tx.CategoryID, tx.Amount, tx.Currency,
		tx.ToAmount, tx.ToCurrency, rateText, tx.BaseAmount, tx.Note, tx.Tags,
	).Scan(&tx.ID, &tx.CreatedAt)
	if err != nil {
		return fmt.Errorf("create transaction: %w", err)
	}

	return nil
}

func (r repository) List(ctx context.Context, filter Filter) ([]entities.Transaction, error) {
	query := `SELECT ` + txColumns + ` FROM transactions WHERE 1 = 1`
	args := []any{}
	add := func(cond string, val any) {
		args = append(args, val)
		query += fmt.Sprintf(cond, len(args))
	}

	if filter.AccountID != nil {
		// an account matches either leg
		args = append(args, *filter.AccountID)
		query += fmt.Sprintf(` AND (from_account_id = $%d OR to_account_id = $%d)`, len(args), len(args))
	}
	if filter.CategoryID != nil {
		add(` AND category_id = $%d`, *filter.CategoryID)
	}
	if filter.Type != nil {
		add(` AND type = $%d`, *filter.Type)
	}
	if filter.DateFrom != nil {
		add(` AND date >= $%d`, *filter.DateFrom)
	}
	if filter.DateTo != nil {
		add(` AND date <= $%d`, *filter.DateTo)
	}
	if filter.Tag != nil {
		add(` AND tags @> ARRAY[$%d]`, *filter.Tag)
	}
	if filter.Query != nil {
		add(` AND note ILIKE '%%' || $%d || '%%'`, *filter.Query)
	}

	query += ` ORDER BY date DESC, created_at DESC`
	add(` LIMIT $%d`, limitOrDefault(filter.Limit))
	add(` OFFSET $%d`, maxInt(filter.Offset, 0))

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list transactions: %w", err)
	}
	defer rows.Close()

	var txns []entities.Transaction
	for rows.Next() {
		t, scanErr := scanTransaction(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("list transactions: %w", scanErr)
		}

		txns = append(txns, t)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("list transactions: %w", err)
	}

	return txns, nil
}

func (r repository) Get(ctx context.Context, id uuid.UUID) (*entities.Transaction, error) {
	query := `SELECT ` + txColumns + ` FROM transactions WHERE id = $1`

	t, err := scanTransaction(r.db.Pool.QueryRow(ctx, query, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get transaction: %w", err)
	}

	return &t, nil
}

func (r repository) Delete(ctx context.Context, id uuid.UUID) error {
	res, err := r.db.Pool.Exec(ctx, `DELETE FROM transactions WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete transaction: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

func limitOrDefault(limit int) int {
	if limit <= 0 {
		return 100
	}

	return limit
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}

	return b
}
