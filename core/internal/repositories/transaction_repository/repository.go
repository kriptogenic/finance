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
	// ListBySplitGroup returns every leg of a split, ordered with the expense first.
	ListBySplitGroup(ctx context.Context, group uuid.UUID) ([]entities.Transaction, error)
	Get(ctx context.Context, id uuid.UUID) (*entities.Transaction, error)
	// Update replaces every mutable column of tx.ID (a full edit); id and
	// created_at are preserved.
	Update(ctx context.Context, tx *entities.Transaction) error
	SetCategory(ctx context.Context, id, categoryID uuid.UUID) error
	Delete(ctx context.Context, id uuid.UUID) error
	// Ingest idempotently inserts by external_id; created=false means it already existed.
	Ingest(ctx context.Context, tx *entities.Transaction) (created bool, err error)
	// ByExternalID returns the transaction with this external_id, or ErrNotFound.
	ByExternalID(ctx context.Context, externalID string) (*entities.Transaction, error)
	// ConsumedTransfer returns the transfer that swallowed this external_id as a
	// paired leg, or ErrNotFound when the id was never consumed.
	ConsumedTransfer(ctx context.Context, externalID string) (*entities.Transaction, error)
	// FindTransferMate returns the closest committed leg that pairs with an
	// ingested leg into a transfer (opposite type, same amount, distinct account,
	// near in time), or ErrNotFound.
	FindTransferMate(ctx context.Context, q MateQuery) (*entities.Transaction, error)
	// MergeIntoTransfer rewrites survivorID's row into transfer in place and
	// records consumedExternalID in consumed_legs, atomically. transfer is
	// back-filled with the surviving row's id/created_at.
	MergeIntoTransfer(ctx context.Context, survivorID uuid.UUID, transfer *entities.Transaction, consumedExternalID string) error
	// CountUncategorized returns how many transactions sit in the Uncategorized buckets.
	CountUncategorized(ctx context.Context) (int, error)
}

// MateQuery describes the opposite leg to look for when pairing an ingested
// expense/income into a transfer.
type MateQuery struct {
	OppositeType     entities.TransactionType
	AmountMinor      int64
	Currency         string
	ExcludeAccountID uuid.UUID // the ingested leg's account; the mate must differ
	From             time.Time // earliest occurred-at to consider
	To               time.Time // latest occurred-at to consider
	Around           time.Time // the ingested leg's occurred-at; closest wins
}

// uncategorizedBuckets selects the ids of the built-in Uncategorized categories.
const uncategorizedBuckets = `SELECT id FROM categories
	WHERE system_key IN ('uncategorized_expense', 'uncategorized_income')`

type repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) Repository {
	return &repository{db: db}
}

// rate_to_base is cast to text so it scans into fx.Rate without relying on pgx
// numeric decoding. amount/to_amount/base_amount are money_t composites.
const txColumns = `id, date, type, from_account_id, to_account_id, category_id,
	amount, to_amount, rate_to_base::text AS rate_to_base, base_amount,
	note, tags, created_at, external_id, transfer_group_id, split_group_id,
	(SELECT id FROM receipts WHERE receipts.transaction_id = transactions.id) AS receipt_id`

func (r repository) Create(ctx context.Context, tx *entities.Transaction) error {
	var rateText *string
	if tx.RateToBase != nil {
		s := tx.RateToBase.String()
		rateText = &s
	}

	const query = `
		INSERT INTO transactions
			(date, type, from_account_id, to_account_id, category_id, amount,
			 to_amount, rate_to_base, base_amount, note, tags,
			 external_id, transfer_group_id, split_group_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8::numeric, $9, $10, $11, $12, $13, $14)
		RETURNING id, created_at`

	err := r.db.Pool.QueryRow(ctx, query,
		tx.Date, tx.Type, tx.FromAccountID, tx.ToAccountID, tx.CategoryID, tx.Amount,
		tx.ToAmount, rateText, tx.BaseAmount, tx.Note, tx.Tags,
		tx.ExternalID, tx.TransferGroupID, tx.SplitGroupID,
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
			(date, type, from_account_id, to_account_id, category_id, amount,
			 to_amount, rate_to_base, base_amount, note, tags,
			 external_id, transfer_group_id, split_group_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8::numeric, $9, $10, $11, $12, $13, $14)
		ON CONFLICT (external_id) WHERE external_id IS NOT NULL DO NOTHING
		RETURNING id, created_at`

	err = r.db.Pool.QueryRow(ctx, query,
		tx.Date, tx.Type, tx.FromAccountID, tx.ToAccountID, tx.CategoryID, tx.Amount,
		tx.ToAmount, rateText, tx.BaseAmount, tx.Note, tx.Tags,
		tx.ExternalID, tx.TransferGroupID, tx.SplitGroupID,
	).Scan(&tx.ID, &tx.CreatedAt)

	if err == nil {
		return true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return false, fmt.Errorf("ingest transaction: %w", err)
	}

	// conflict: the external_id already exists — return the stored transaction
	existing, getErr := r.queryOne(ctx,
		`SELECT `+txColumns+` FROM transactions WHERE external_id = $1`, *tx.ExternalID)
	if getErr != nil {
		return false, fmt.Errorf("ingest lookup: %w", getErr)
	}
	*tx = *existing

	return false, nil
}

func (r repository) ByExternalID(ctx context.Context, externalID string) (*entities.Transaction, error) {
	tx, err := r.queryOne(ctx,
		`SELECT `+txColumns+` FROM transactions WHERE external_id = $1`, externalID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("transaction by external_id: %w", err)
	}

	return tx, nil
}

func (r repository) ConsumedTransfer(ctx context.Context, externalID string) (*entities.Transaction, error) {
	tx, err := r.queryOne(ctx,
		`SELECT `+txColumns+` FROM transactions
		 WHERE id = (SELECT transaction_id FROM consumed_legs WHERE external_id = $1)`, externalID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("consumed transfer lookup: %w", err)
	}

	return tx, nil
}

func (r repository) FindTransferMate(ctx context.Context, q MateQuery) (*entities.Transaction, error) {
	// A mate is an ingested, non-split leg of the opposite type with the same
	// amount on a different account, within the pairing window. The closest in
	// time wins, matching the old in-buffer behaviour.
	const query = `SELECT ` + txColumns + ` FROM transactions
		WHERE type = $1
		  AND (amount).amount = $2
		  AND (amount).currency = $3
		  AND external_id IS NOT NULL
		  AND split_group_id IS NULL
		  AND COALESCE(from_account_id, to_account_id) <> $4
		  AND date BETWEEN $5 AND $6
		ORDER BY abs(extract(epoch FROM date - $7)) ASC
		LIMIT 1`

	tx, err := r.queryOne(ctx, query,
		q.OppositeType, q.AmountMinor, q.Currency, q.ExcludeAccountID, q.From, q.To, q.Around)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find transfer mate: %w", err)
	}

	return tx, nil
}

func (r repository) MergeIntoTransfer(ctx context.Context, survivorID uuid.UUID, transfer *entities.Transaction, consumedExternalID string) error {
	var rateText *string
	if transfer.RateToBase != nil {
		s := transfer.RateToBase.String()
		rateText = &s
	}

	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin merge: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const update = `
		UPDATE transactions SET
			date = $2, type = 'transfer', from_account_id = $3, to_account_id = $4,
			category_id = NULL, amount = $5, rate_to_base = $6::numeric, base_amount = $7,
			note = NULL, tags = $8, transfer_group_id = $9
		WHERE id = $1
		RETURNING id, created_at`

	err = tx.QueryRow(ctx, update,
		survivorID, transfer.Date, transfer.FromAccountID, transfer.ToAccountID,
		transfer.Amount, rateText, transfer.BaseAmount, transfer.Tags, transfer.TransferGroupID,
	).Scan(&transfer.ID, &transfer.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("merge update: %w", err)
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO consumed_legs (external_id, transaction_id) VALUES ($1, $2)`,
		consumedExternalID, survivorID)
	if err != nil {
		return fmt.Errorf("record consumed leg: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit merge: %w", err)
	}

	return nil
}

func (r repository) CountUncategorized(ctx context.Context) (int, error) {
	var n int
	err := r.db.Pool.QueryRow(ctx,
		`SELECT count(*) FROM transactions WHERE category_id IN (`+uncategorizedBuckets+`)`).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count uncategorized: %w", err)
	}

	return n, nil
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
		query += ` AND category_id IN (` + uncategorizedBuckets + `)`
	}

	query += ` ORDER BY date DESC, created_at DESC`
	add(` LIMIT $%d`, limitOrDefault(filter.Limit))
	add(` OFFSET $%d`, maxInt(filter.Offset, 0))

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list transactions: %w", err)
	}

	txns, err := pgx.CollectRows(rows, pgx.RowToStructByName[entities.Transaction])
	if err != nil {
		return nil, fmt.Errorf("list transactions: %w", err)
	}

	return txns, nil
}

func (r repository) ListBySplitGroup(ctx context.Context, group uuid.UUID) ([]entities.Transaction, error) {
	// expense (the main leg) first, then the per-person transfer legs
	query := `SELECT ` + txColumns + ` FROM transactions
		WHERE split_group_id = $1
		ORDER BY (type = 'expense') DESC, created_at`

	rows, err := r.db.Pool.Query(ctx, query, group)
	if err != nil {
		return nil, fmt.Errorf("list split group: %w", err)
	}

	txns, err := pgx.CollectRows(rows, pgx.RowToStructByName[entities.Transaction])
	if err != nil {
		return nil, fmt.Errorf("list split group: %w", err)
	}

	return txns, nil
}

func (r repository) Get(ctx context.Context, id uuid.UUID) (*entities.Transaction, error) {
	query := `SELECT ` + txColumns + ` FROM transactions WHERE id = $1`

	t, err := r.queryOne(ctx, query, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get transaction: %w", err)
	}

	return t, nil
}

func (r repository) queryOne(ctx context.Context, query string, args ...any) (*entities.Transaction, error) {
	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	t, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[entities.Transaction])
	if err != nil {
		return nil, err
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
			amount = $7, to_amount = $8,
			rate_to_base = $9::numeric, base_amount = $10, note = $11, tags = $12
		WHERE id = $1`

	res, err := r.db.Pool.Exec(ctx, query,
		tx.ID, tx.Date, tx.Type, tx.FromAccountID, tx.ToAccountID, tx.CategoryID,
		tx.Amount, tx.ToAmount, rateText, tx.BaseAmount,
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
