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
)

type Repository interface {
	// Create inserts a new receipt header (typically in the pending state).
	Create(ctx context.Context, r *entities.Receipt) error
	// SetStatus updates only the status (and optional error reason).
	SetStatus(ctx context.Context, id uuid.UUID, status entities.ReceiptStatus, errMsg *string) error
	// SetPhotoKey records the stored photo's object key.
	SetPhotoKey(ctx context.Context, id uuid.UUID, key string) error
	// SetRawHTML stores the scraped HTML and flips status to html_received.
	SetRawHTML(ctx context.Context, id uuid.UUID, html string) error
	// SaveParsed writes all parsed header fields + items and marks success.
	SaveParsed(ctx context.Context, r *entities.Receipt) error
	// SetTransaction links (txID non-nil) or unlinks (txID nil) the receipt's
	// transaction. Returns ErrAlreadyLinked if txID is taken by another receipt.
	SetTransaction(ctx context.Context, id uuid.UUID, txID *uuid.UUID) error
	// FindAutoLinkCandidate returns the id of the single unlinked UZS expense
	// matching amount within [from, to]; nil when none or ambiguous.
	FindAutoLinkCandidate(ctx context.Context, amount int64, from, to time.Time) (*uuid.UUID, error)
	Get(ctx context.Context, id uuid.UUID) (*entities.Receipt, error)
	List(ctx context.Context, page, limit int) ([]entities.Receipt, error)
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
		return fmt.Errorf("create receipt: %w", err)
	}

	return nil
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

func (r repository) SetRawHTML(ctx context.Context, id uuid.UUID, html string) error {
	if _, err := r.db.Pool.Exec(ctx,
		`UPDATE receipts SET raw_html = $2, status = $3 WHERE id = $1`,
		id, html, entities.ReceiptHTMLReceived,
	); err != nil {
		return fmt.Errorf("set receipt html: %w", err)
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

const headerCols = `
	id, qr_url, status, error, terminal_id, receipt_seq, fiscal_sign, received_at,
	receipt_type, merchant_name, merchant_tin, merchant_address, device_name, serial_number,
	card_type, merchant_lat, merchant_lng, paid_cash, paid_card, total_amount, total_vat,
	photo_key, scraped_at, created_at, transaction_id`

func scanHeader(row pgx.Row, rec *entities.Receipt) error {
	return row.Scan(
		&rec.ID, &rec.QRURL, &rec.Status, &rec.Error, &rec.TerminalID, &rec.ReceiptSeq, &rec.FiscalSign, &rec.ReceivedAt,
		&rec.ReceiptType, &rec.MerchantName, &rec.MerchantTIN, &rec.MerchantAddress, &rec.DeviceName, &rec.SerialNumber,
		&rec.CardType, &rec.MerchantLat, &rec.MerchantLng, &rec.PaidCash, &rec.PaidCard, &rec.TotalAmount, &rec.TotalVAT,
		&rec.PhotoKey, &rec.ScrapedAt, &rec.CreatedAt, &rec.TransactionID,
	)
}

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
		WHERE t.type = 'expense' AND t.currency = $1 AND t.amount = $2
		  AND t.date BETWEEN $3 AND $4 AND r.id IS NULL
		LIMIT 2`

	rows, err := r.db.Pool.Query(ctx, query, entities.ReceiptCurrency, amount, from, to)
	if err != nil {
		return nil, fmt.Errorf("find auto-link candidate: %w", err)
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan auto-link candidate: %w", err)
		}
		ids = append(ids, id)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("find auto-link candidate: %w", err)
	}

	if len(ids) != 1 { // none or ambiguous
		return nil, nil
	}

	return &ids[0], nil
}

func (r repository) Get(ctx context.Context, id uuid.UUID) (*entities.Receipt, error) {
	var rec entities.Receipt
	err := scanHeader(r.db.Pool.QueryRow(ctx, `SELECT `+headerCols+` FROM receipts WHERE id = $1`, id), &rec)
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

func (r repository) items(ctx context.Context, id uuid.UUID) ([]entities.ReceiptItem, error) {
	const query = `
		SELECT name, quantity, price, vat_amount, vat_rate, discount, other,
		       barcode, ikpu_code, ikpu_name, unit, marking_code, consignor_tin
		FROM receipt_items WHERE receipt_id = $1 ORDER BY id`

	rows, err := r.db.Pool.Query(ctx, query, id)
	if err != nil {
		return nil, fmt.Errorf("list receipt items: %w", err)
	}
	defer rows.Close()

	var items []entities.ReceiptItem
	for rows.Next() {
		var it entities.ReceiptItem
		if err = rows.Scan(
			&it.Name, &it.Quantity, &it.Price, &it.VATAmount, &it.VATRate, &it.Discount, &it.Other,
			&it.Barcode, &it.IKPUCode, &it.IKPUName, &it.Unit, &it.MarkingCode, &it.ConsignorTIN,
		); err != nil {
			return nil, fmt.Errorf("scan receipt item: %w", err)
		}
		items = append(items, it)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("list receipt items: %w", err)
	}

	return items, nil
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
	defer rows.Close()

	var receipts []entities.Receipt
	for rows.Next() {
		var rec entities.Receipt
		if err = scanHeader(rows, &rec); err != nil {
			return nil, fmt.Errorf("scan receipt: %w", err)
		}
		receipts = append(receipts, rec)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("list receipts: %w", err)
	}

	return receipts, nil
}
