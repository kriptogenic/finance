package receiptrepository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"finance/internal/entities"
	"finance/pkg/database"
)

var (
	ErrNotFound = errors.New("receipt not found")
	// ErrAlreadyLinked means the target transaction is linked to another receipt.
	ErrAlreadyLinked = errors.New("transaction already linked to a receipt")
	// ErrDuplicate means a receipt with the same fiscal identity already exists.
	ErrDuplicate = errors.New("receipt already exists")
)

type Repository interface {
	// Create inserts a new receipt header (typically in the pending state).
	// Returns ErrDuplicate when the fiscal identity already exists.
	Create(ctx context.Context, r *entities.Receipt) error
	// FindByFiscal returns the receipt matching the fiscal triple, or ErrNotFound.
	FindByFiscal(ctx context.Context, terminalID *string, seq *int, fiscalSign *string) (*entities.Receipt, error)
	// SetStatus updates only the status (and optional error reason).
	SetStatus(ctx context.Context, id uuid.UUID, status entities.ReceiptStatus, errMsg *string) error
	// SetPhotoKey records the stored photo's object key.
	SetPhotoKey(ctx context.Context, id uuid.UUID, key string) error
	// SetRawPayload stores the fetched payload and flips status to html_received.
	SetRawPayload(ctx context.Context, id uuid.UUID, payload string) error
	// SaveParsed writes all parsed header fields + items and marks success.
	SaveParsed(ctx context.Context, r *entities.Receipt) error
	// SetTransaction links (txID non-nil) or unlinks (txID nil) the receipt's
	// transaction. Returns ErrAlreadyLinked if txID is taken by another receipt.
	SetTransaction(ctx context.Context, id uuid.UUID, txID *uuid.UUID) error
	// FindAutoLinkCandidate returns the id of the single unlinked UZS expense
	// matching amount within [from, to]; nil when none or ambiguous.
	FindAutoLinkCandidate(ctx context.Context, amount int64, from, to time.Time) (*uuid.UUID, error)
	Get(ctx context.Context, id uuid.UUID) (*entities.Receipt, error)
	GetRawPayload(ctx context.Context, id uuid.UUID) (string, error)
	List(ctx context.Context, page, limit int) ([]entities.Receipt, error)
	// Delete removes the receipt (items cascade). Returns ErrNotFound if missing.
	Delete(ctx context.Context, id uuid.UUID) error
}

type repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) Repository {
	return &repository{db: db}
}

func (r repository) Create(ctx context.Context, rec *entities.Receipt) error {
	const query = `
		INSERT INTO receipts (qr_url, status, terminal_id, receipt_seq, fiscal_sign, received_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at`

	err := r.db.Pool.QueryRow(ctx, query,
		rec.QRURL, rec.Status, rec.TerminalID, rec.ReceiptSeq, rec.FiscalSign, rec.ReceivedAt,
	).Scan(&rec.ID, &rec.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrDuplicate
		}

		return fmt.Errorf("create receipt: %w", err)
	}

	return nil
}

func (r repository) FindByFiscal(ctx context.Context, terminalID *string, seq *int, fiscalSign *string) (*entities.Receipt, error) {
	const query = `SELECT ` + headerCols + `
		FROM receipts
		WHERE terminal_id = $1 AND receipt_seq = $2 AND fiscal_sign = $3
		ORDER BY (transaction_id IS NOT NULL) DESC, created_at ASC
		LIMIT 1`

	rows, err := r.db.Pool.Query(ctx, query, terminalID, seq, fiscalSign)
	if err != nil {
		return nil, fmt.Errorf("find receipt by fiscal: %w", err)
	}

	rec, err := pgx.CollectOneRow(rows, pgx.RowToStructByNameLax[entities.Receipt])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find receipt by fiscal: %w", err)
	}

	return &rec, nil
}

func (r repository) SetStatus(ctx context.Context, id uuid.UUID, status entities.ReceiptStatus, errMsg *string) error {
	if _, err := r.db.Pool.Exec(ctx,
		`UPDATE receipts SET status = $2, error = $3 WHERE id = $1`,
		id, status, errMsg,
	); err != nil {
		return fmt.Errorf("set receipt status: %w", err)
	}

	return nil
}

func (r repository) SetPhotoKey(ctx context.Context, id uuid.UUID, key string) error {
	if _, err := r.db.Pool.Exec(ctx,
		`UPDATE receipts SET photo_key = $2 WHERE id = $1`, id, key,
	); err != nil {
		return fmt.Errorf("set receipt photo key: %w", err)
	}

	return nil
}

func (r repository) SetRawPayload(ctx context.Context, id uuid.UUID, payload string) error {
	if _, err := r.db.Pool.Exec(ctx,
		`UPDATE receipts SET raw_payload = $2, status = $3 WHERE id = $1`,
		id, payload, entities.ReceiptHTMLReceived,
	); err != nil {
		return fmt.Errorf("set receipt payload: %w", err)
	}

	return nil
}

func (r repository) SaveParsed(ctx context.Context, rec *entities.Receipt) (err error) {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("save parsed begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const header = `
		UPDATE receipts SET
			status = 'success', error = NULL, scraped_at = now(),
			receipt_type = $2, merchant_name = $3, merchant_tin = $4, merchant_address = $5,
			device_name = $6, serial_number = $7, card_type = $8,
			merchant_lat = $9, merchant_lng = $10,
			paid_cash = $11, paid_card = $12, total_amount = $13, total_vat = $14,
			received_at = COALESCE($15, received_at)
		WHERE id = $1`

	if _, err = tx.Exec(ctx, header,
		rec.ID, rec.ReceiptType, rec.MerchantName, rec.MerchantTIN, rec.MerchantAddress,
		rec.DeviceName, rec.SerialNumber, rec.CardType, rec.MerchantLat, rec.MerchantLng,
		rec.PaidCash, rec.PaidCard, rec.TotalAmount, rec.TotalVAT, rec.ReceivedAt,
	); err != nil {
		return fmt.Errorf("save parsed header: %w", err)
	}

	if _, err = tx.Exec(ctx, `DELETE FROM receipt_items WHERE receipt_id = $1`, rec.ID); err != nil {
		return fmt.Errorf("save parsed clear items: %w", err)
	}

	const item = `
		INSERT INTO receipt_items
			(receipt_id, name, quantity, price, vat_amount, vat_rate, discount, other,
			 barcode, ikpu_code, ikpu_name, unit, marking_code, consignor_tin)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`

	for _, it := range rec.Items {
		if _, err = tx.Exec(ctx, item,
			rec.ID, it.Name, it.Quantity, it.Price, it.VATAmount, it.VATRate, it.Discount, it.Other,
			it.Barcode, it.IKPUCode, it.IKPUName, it.Unit, it.MarkingCode, it.ConsignorTIN,
		); err != nil {
			return fmt.Errorf("save parsed item: %w", err)
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("save parsed commit: %w", err)
	}

	return nil
}

// raw_payload is omitted (fetched separately via GetRawPayload); reads scan into
// the Receipt header with the lax mapper, leaving RawPayload and Items unset.
const headerCols = `
	id, qr_url, status, error, terminal_id, receipt_seq, fiscal_sign, received_at,
	receipt_type, merchant_name, merchant_tin, merchant_address, device_name, serial_number,
	card_type, merchant_lat, merchant_lng, paid_cash, paid_card, total_amount, total_vat,
	photo_key, scraped_at, created_at, transaction_id`

func (r repository) SetTransaction(ctx context.Context, id uuid.UUID, txID *uuid.UUID) error {
	_, err := r.db.Pool.Exec(ctx, `UPDATE receipts SET transaction_id = $2 WHERE id = $1`, id, txID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrAlreadyLinked
		}

		return fmt.Errorf("set receipt transaction: %w", err)
	}

	return nil
}

func (r repository) FindAutoLinkCandidate(ctx context.Context, amount int64, from, to time.Time) (*uuid.UUID, error) {
	const query = `
		SELECT t.id FROM transactions t
		LEFT JOIN receipts r ON r.transaction_id = t.id
		WHERE t.type = 'expense' AND (t.amount).currency = $1 AND (t.amount).amount = $2
		  AND t.date BETWEEN $3 AND $4 AND r.id IS NULL
		LIMIT 2`

	rows, err := r.db.Pool.Query(ctx, query, entities.ReceiptCurrency, amount, from, to)
	if err != nil {
		return nil, fmt.Errorf("find auto-link candidate: %w", err)
	}

	ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		return nil, fmt.Errorf("find auto-link candidate: %w", err)
	}

	if len(ids) != 1 { // none or ambiguous
		return nil, nil
	}

	return &ids[0], nil
}

func (r repository) Get(ctx context.Context, id uuid.UUID) (*entities.Receipt, error) {
	rows, err := r.db.Pool.Query(ctx, `SELECT `+headerCols+` FROM receipts WHERE id = $1`, id)
	if err != nil {
		return nil, fmt.Errorf("get receipt: %w", err)
	}

	rec, err := pgx.CollectOneRow(rows, pgx.RowToStructByNameLax[entities.Receipt])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get receipt: %w", err)
	}

	items, err := r.items(ctx, id)
	if err != nil {
		return nil, err
	}
	rec.Items = items

	return &rec, nil
}

func (r repository) GetRawPayload(ctx context.Context, id uuid.UUID) (string, error) {
	var payload *string
	err := r.db.Pool.QueryRow(ctx, `SELECT raw_payload FROM receipts WHERE id = $1`, id).Scan(&payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("get receipt payload: %w", err)
	}
	if payload == nil {
		return "", nil
	}

	return *payload, nil
}

func (r repository) items(ctx context.Context, id uuid.UUID) ([]entities.ReceiptItem, error) {
	const query = `
		SELECT name, quantity, price, vat_amount, vat_rate, discount, other,
		       barcode, ikpu_code, ikpu_name, unit, marking_code, consignor_tin
		FROM receipt_items WHERE receipt_id = $1 ORDER BY id`

	rows, err := r.db.Pool.Query(ctx, query, id)
	if err != nil {
		return nil, fmt.Errorf("list receipt items: %w", err)
	}

	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[entities.ReceiptItem])
	if err != nil {
		return nil, fmt.Errorf("list receipt items: %w", err)
	}

	return items, nil
}

func (r repository) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Pool.Exec(ctx, `DELETE FROM receipts WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete receipt: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

func (r repository) List(ctx context.Context, page, limit int) ([]entities.Receipt, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	offset := (page - 1) * limit

	rows, err := r.db.Pool.Query(ctx,
		`SELECT `+headerCols+` FROM receipts ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
		limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list receipts: %w", err)
	}

	receipts, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[entities.Receipt])
	if err != nil {
		return nil, fmt.Errorf("list receipts: %w", err)
	}

	return receipts, nil
}
