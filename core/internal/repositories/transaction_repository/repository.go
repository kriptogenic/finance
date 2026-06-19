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
	AccountID     *uuid.UUID
	CategoryID    *uuid.UUID
	Type          *entities.TransactionType
	DateFrom      *time.Time
	DateTo        *time.Time
	Tag           *string
	Query         *string
	Uncategorized bool
	Limit         int
	Offset        int
}

type Repository interface {
	Create(ctx context.Context, tx *entities.Transaction) error
	List(ctx context.Context, filter Filter) ([]entities.Transaction, error)
	Get(ctx context.Context, id uuid.UUID) (*entities.Transaction, error)
	// Update replaces every mutable column of tx.ID (a full edit); id and
	// created_at are preserved.
	Update(ctx context.Context, tx *entities.Transaction) error
	SetCategory(ctx context.Context, id, categoryID uuid.UUID) error
	Delete(ctx context.Context, id uuid.UUID) error
	// Ingest idempotently inserts by external_id; created=false means it already existed.
	Ingest(ctx context.Context, tx *entities.Transaction) (created bool, err error)
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
	note, tags, created_at, external_id, transfer_group_id`

func scanTransaction(row pgx.Row) (entities.Transaction, error) {
	var (
		t        entities.Transaction
		rateText *string
	)
	err := row.Scan(
		&t.ID, &t.Date, &t.Type, &t.FromAccountID, &t.ToAccountID, &t.CategoryID,
		&t.Amount, &t.Currency, &t.ToAmount, &t.ToCurrency, &rateText, &t.BaseAmount,
		&t.Note, &t.Tags, &t.CreatedAt, &t.ExternalID, &t.TransferGroupID,
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
			 to_amount, to_currency, rate_to_base, base_amount, note, tags,
			 external_id, transfer_group_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::numeric, $11, $12, $13, $14, $15)
		RETURNING id, created_at`

	err := r.db.Pool.QueryRow(ctx, query,
		tx.Date, tx.Type, tx.FromAccountID, tx.ToAccountID, tx.CategoryID, tx.Amount, tx.Currency,
		tx.ToAmount, tx.ToCurrency, rateText, tx.BaseAmount, tx.Note, tx.Tags,
		tx.ExternalID, tx.TransferGroupID,
	).Scan(&tx.ID, &tx.CreatedAt)
	if err != nil {
		return fmt.Errorf("create transaction: %w", err)
	}

	return nil
}

// Ingest idempotently inserts a transaction keyed by external_id. It returns
// created=false (and the existing row) when external_id was already ingested, so
// retries never duplicate. tx.ExternalID must be set.
func (r repository) Ingest(ctx context.Context, tx *entities.Transaction) (created bool, err error) {
	if tx.ExternalID == nil || *tx.ExternalID == "" {
		return false, errors.New("ingest requires an external_id")
	}

	var rateText *string
	if tx.RateToBase != nil {
		s := tx.RateToBase.String()
		rateText = &s
	}

	const query = `
		INSERT INTO transactions
			(date, type, from_account_id, to_account_id, category_id, amount, currency,
			 to_amount, to_currency, rate_to_base, base_amount, note, tags,
			 external_id, transfer_group_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::numeric, $11, $12, $13, $14, $15)
		ON CONFLICT (external_id) WHERE external_id IS NOT NULL DO NOTHING
		RETURNING id, created_at`

	err = r.db.Pool.QueryRow(ctx, query,
		tx.Date, tx.Type, tx.FromAccountID, tx.ToAccountID, tx.CategoryID, tx.Amount, tx.Currency,
		tx.ToAmount, tx.ToCurrency, rateText, tx.BaseAmount, tx.Note, tx.Tags,
		tx.ExternalID, tx.TransferGroupID,
	).Scan(&tx.ID, &tx.CreatedAt)

	if err == nil {
		return true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return false, fmt.Errorf("ingest transaction: %w", err)
	}

	// conflict: the external_id already exists — return the stored transaction
	existing, getErr := scanTransaction(r.db.Pool.QueryRow(ctx,
		`SELECT `+txColumns+` FROM transactions WHERE external_id = $1`, *tx.ExternalID))
	if getErr != nil {
		return false, fmt.Errorf("ingest lookup: %w", getErr)
	}
	*tx = existing

	return false, nil
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
	if filter.Uncategorized {
		query += ` AND category_id IN (SELECT id FROM categories
			WHERE system_key IN ('uncategorized_expense', 'uncategorized_income'))`
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

func (r repository) Update(ctx context.Context, tx *entities.Transaction) error {
	var rateText *string
	if tx.RateToBase != nil {
		s := tx.RateToBase.String()
		rateText = &s
	}

	const query = `
		UPDATE transactions SET
			date = $2, type = $3, from_account_id = $4, to_account_id = $5, category_id = $6,
			amount = $7, currency = $8, to_amount = $9, to_currency = $10,
			rate_to_base = $11::numeric, base_amount = $12, note = $13, tags = $14
		WHERE id = $1`

	res, err := r.db.Pool.Exec(ctx, query,
		tx.ID, tx.Date, tx.Type, tx.FromAccountID, tx.ToAccountID, tx.CategoryID,
		tx.Amount, tx.Currency, tx.ToAmount, tx.ToCurrency, rateText, tx.BaseAmount,
		tx.Note, tx.Tags,
	)
	if err != nil {
		return fmt.Errorf("update transaction: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
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

func (r repository) SetCategory(ctx context.Context, id, categoryID uuid.UUID) error {
	res, err := r.db.Pool.Exec(ctx,
		`UPDATE transactions SET category_id = $2 WHERE id = $1`, id, categoryID)
	if err != nil {
		return fmt.Errorf("set transaction category: %w", err)
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
