package entities

import (
	"time"

	"github.com/google/uuid"
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
	ID     uuid.UUID
	QRURL  string
	Status ReceiptStatus
	Error  *string

	// From the QR url query params.
	TerminalID *string
	ReceiptSeq *int
	FiscalSign *string
	ReceivedAt *time.Time

	// Parsed header.
	ReceiptType     *string
	MerchantName    *string
	MerchantTIN     *string
	MerchantAddress *string
	DeviceName      *string
	SerialNumber    *string
	CardType        *string
	MerchantLat     *string
	MerchantLng     *string

	// Parsed totals (tiyin).
	PaidCash    int64
	PaidCard    int64
	TotalAmount int64
	TotalVAT    int64

	PhotoKey  *string
	RawHTML   *string
	ScrapedAt *time.Time
	CreatedAt time.Time

	// linked expense transaction, if any (1:1)
	TransactionID *uuid.UUID

	Items []ReceiptItem
}

// ReceiptItem is one line of a receipt. Monetary fields are tiyin (UZS).
type ReceiptItem struct {
	Name         string
	Quantity     string
	Price        int64
	VATAmount    int64
	VATRate      int
	Discount     int64
	Other        int64
	Barcode      *string
	IKPUCode     *string
	IKPUName     *string
	Unit         *string
	MarkingCode  *string
	ConsignorTIN *string
}
