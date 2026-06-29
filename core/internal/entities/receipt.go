package entities

import (
	"time"

	"github.com/google/uuid"

	"finance/pkg/money"
)

// ReceiptStatus is the lifecycle of an async receipt scrape+parse.
type ReceiptStatus string

const (
	ReceiptPending      ReceiptStatus = "pending"
	ReceiptHTMLReceived ReceiptStatus = "html_received"
	ReceiptSuccess      ReceiptStatus = "success"
	ReceiptFailed       ReceiptStatus = "failed"
)

// ReceiptCurrency is the only currency Uzbekistan fiscal receipts use.
const ReceiptCurrency = "UZS"

// Receipt is a scanned fiscal receipt: the QR url, stored photo, and the data
// scraped+parsed from ofd.soliq.uz. Monetary fields are integer minor units
// (tiyin) in UZS.
type Receipt struct {
	ID     uuid.UUID     `db:"id"`
	QRURL  string        `db:"qr_url"`
	Status ReceiptStatus `db:"status"`
	Error  *string       `db:"error"`

	// From the QR url query params.
	TerminalID *string    `db:"terminal_id"`
	ReceiptSeq *int       `db:"receipt_seq"`
	FiscalSign *string    `db:"fiscal_sign"`
	ReceivedAt *time.Time `db:"received_at"`

	// Parsed header.
	ReceiptType     *string `db:"receipt_type"`
	MerchantName    *string `db:"merchant_name"`
	MerchantTIN     *string `db:"merchant_tin"`
	MerchantAddress *string `db:"merchant_address"`
	DeviceName      *string `db:"device_name"`
	SerialNumber    *string `db:"serial_number"`
	CardType        *string `db:"card_type"`
	MerchantLat     *string `db:"merchant_lat"`
	MerchantLng     *string `db:"merchant_lng"`

	// Parsed totals (UZS).
	PaidCash    money.Money `db:"paid_cash"`
	PaidCard    money.Money `db:"paid_card"`
	TotalAmount money.Money `db:"total_amount"`
	TotalVAT    money.Money `db:"total_vat"`

	PhotoKey   *string    `db:"photo_key"`
	RawPayload *string    `db:"raw_payload"` // not in the header read; fetched separately
	ScrapedAt  *time.Time `db:"scraped_at"`
	CreatedAt  time.Time  `db:"created_at"`

	// linked expense transaction, if any (1:1)
	TransactionID *uuid.UUID `db:"transaction_id"`

	Items []ReceiptItem `db:"-"` // child relation, loaded separately
}

// ReceiptItem is one line of a receipt. Monetary fields are UZS.
type ReceiptItem struct {
	Name         string      `db:"name"`
	Quantity     string      `db:"quantity"`
	Price        money.Money `db:"price"`
	VATAmount    money.Money `db:"vat_amount"`
	VATRate      int         `db:"vat_rate"`
	Discount     money.Money `db:"discount"`
	Other        money.Money `db:"other"`
	Barcode      *string     `db:"barcode"`
	IKPUCode     *string     `db:"ikpu_code"`
	IKPUName     *string     `db:"ikpu_name"`
	Unit         *string     `db:"unit"`
	MarkingCode  *string     `db:"marking_code"`
	ConsignorTIN *string     `db:"consignor_tin"`
}
