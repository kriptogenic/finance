package receipts

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"finance/internal/entities"
	"finance/pkg/money"
)

func uzs(v int64) money.Money { return money.New(v, entities.ReceiptCurrency) }

// soliqZone is Uzbekistan time (UTC+5, no DST). soliq timestamps carry no zone,
// so parse them in this fixed offset rather than defaulting to UTC.
var soliqZone = time.FixedZone("UZT", 5*60*60)

// cardTypeLabels maps soliq's numeric card type to its Uzbek label (from the
// ofd.soliq.uz web client).
var cardTypeLabels = map[int]string{1: "Korporativ", 2: "Shaxsiy", 3: "Ijtimoiy"}

// parseMinor converts a UZS major-unit decimal string (e.g. "5990", "5990.0",
// "641.79") into integer minor units (tiyin) without floating point.
func parseMinor(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")

	intPart, frac, _ := strings.Cut(s, ".")
	frac = (frac + "00")[:2] // pad/truncate to exactly 2 decimal places

	whole, _ := strconv.ParseInt(intPart, 10, 64)
	cents, _ := strconv.ParseInt(frac, 10, 64)
	v := whole*100 + cents
	if neg {
		v = -v
	}

	return v
}

// ParseQRParams extracts the terminal id, receipt sequence, fiscal sign and
// receipt time from the ofd.soliq.uz QR url query (t / r / s / c).
func ParseQRParams(qrURL string) (terminalID *string, seq *int, fiscalSign *string, receivedAt *time.Time) {
	u, err := url.Parse(qrURL)
	if err != nil {
		return nil, nil, nil, nil
	}
	q := u.Query()

	if t := q.Get("t"); t != "" {
		terminalID = &t
	}
	if s := q.Get("s"); s != "" {
		fiscalSign = &s
	}
	if r := q.Get("r"); r != "" {
		if n, err := strconv.Atoi(r); err == nil {
			seq = &n
		}
	}
	if c := q.Get("c"); c != "" {
		// c is a compact timestamp, e.g. 20240102150405.
		if ts, err := time.ParseInLocation("20060102150405", c, soliqZone); err == nil {
			receivedAt = &ts
		}
	}

	return terminalID, seq, fiscalSign, receivedAt
}

// paymentParams are the QR fields the payment API request body needs.
type paymentParams struct {
	TerminalID  string
	PaymentNo   string
	PaymentDate string
	FiscalSign  string
}

// paymentParamsFromQR pulls the payment request fields (t/r/c/s) from the QR
// url. ok is false when any required field is missing.
func paymentParamsFromQR(qrURL string) (p paymentParams, ok bool) {
	u, err := url.Parse(qrURL)
	if err != nil {
		return paymentParams{}, false
	}
	q := u.Query()
	p = paymentParams{
		TerminalID:  q.Get("t"),
		PaymentNo:   q.Get("r"),
		PaymentDate: q.Get("c"),
		FiscalSign:  q.Get("s"),
	}
	if p.TerminalID == "" || p.PaymentNo == "" || p.PaymentDate == "" || p.FiscalSign == "" {
		return paymentParams{}, false
	}

	return p, true
}

// paymentResponse mirrors new-ofd.soliq.uz/api/payment. Monetary fields decode
// as json.Number so parseMinor can convert them without float rounding.
type paymentResponse struct {
	Data    *paymentData `json:"data"`
	Message string       `json:"message"`
	Success bool         `json:"success"`
}

type paymentData struct {
	TIN         json.Number    `json:"tin"`
	PaymentNo   string         `json:"paymentNo"`
	PaymentDate string         `json:"paymentDate"`
	CashTotal   json.Number    `json:"cashTotal"`
	CardTotal   json.Number    `json:"cardTotal"`
	VatTotal    json.Number    `json:"vatTotal"`
	IsRefund    int            `json:"isRefund"`
	Details     []paymentItem  `json:"paymentDetails"`
	ExtraInfo   paymentExtra   `json:"extraInfo"`
	Labels      []paymentLabel `json:"labels"`
	KkmName     string         `json:"kkmName"`
	KkmSerial   string         `json:"kkmSerialNumber"`
}

type paymentItem struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Barcode     string      `json:"barcode"`
	ProductCode string      `json:"productCode"`
	ProductName string      `json:"productName"`
	PackageName string      `json:"packageName"`
	VatPercent  int         `json:"vatPercent"`
	ComitentTin json.Number `json:"comitentTin"`
	Price       json.Number `json:"price"`
	Vat         json.Number `json:"vat"`
	Amount      json.Number `json:"amount"`
	Discount    json.Number `json:"discount"`
	Other       json.Number `json:"other"`
}

type paymentExtra struct {
	CompanyName string `json:"companyName"`
	CardType    *int   `json:"cardType"`
	Latitude    string `json:"latitude"`
	Longitude   string `json:"longitude"`
	Address     string `json:"address"`
}

type paymentLabel struct {
	DetailID string `json:"detailId"`
	Label    string `json:"label"`
}

// ParseJSON maps a new-ofd.soliq.uz payment response into a Receipt.
func ParseJSON(raw []byte) (entities.Receipt, error) {
	var resp paymentResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return entities.Receipt{}, fmt.Errorf("parse payment json: %w", err)
	}
	if resp.Data == nil {
		msg := strings.TrimSpace(resp.Message)
		if msg == "" {
			msg = "no receipt data"
		}

		return entities.Receipt{}, fmt.Errorf("payment response: %s", msg)
	}
	d := resp.Data

	r := entities.Receipt{
		ReceiptType:     strPtr("Savdo cheki/" + refundLabel(d.IsRefund)),
		MerchantName:    strPtr(d.ExtraInfo.CompanyName),
		MerchantTIN:     nonZeroNum(d.TIN),
		MerchantAddress: strPtr(d.ExtraInfo.Address),
		DeviceName:      strPtr(d.KkmName),
		SerialNumber:    strPtr(d.KkmSerial),
		CardType:        cardTypeLabel(d.ExtraInfo.CardType),
		MerchantLat:     strPtr(d.ExtraInfo.Latitude),
		MerchantLng:     strPtr(d.ExtraInfo.Longitude),
		PaidCash:        uzs(parseMinor(d.CashTotal.String())),
		PaidCard:        uzs(parseMinor(d.CardTotal.String())),
		TotalVAT:        uzs(parseMinor(d.VatTotal.String())),
		ReceiptSeq:      parseSeq(d.PaymentNo),
	}
	r.TotalAmount = uzs(r.PaidCash.Minor() + r.PaidCard.Minor())

	// paymentDate is "dd.MM.yyyy HH:mm:ss".
	if ts, err := time.ParseInLocation("02.01.2006 15:04:05", d.PaymentDate, soliqZone); err == nil {
		r.ReceivedAt = &ts
	}

	labels := make(map[string]string, len(d.Labels))
	for _, l := range d.Labels {
		labels[l.DetailID] = l.Label
	}

	r.Items = make([]entities.ReceiptItem, 0, len(d.Details))
	for _, it := range d.Details {
		item := entities.ReceiptItem{
			Name:         it.Name,
			Quantity:     it.Amount.String(),
			Price:        uzs(parseMinor(it.Price.String())),
			VATAmount:    uzs(parseMinor(it.Vat.String())),
			VATRate:      it.VatPercent,
			Discount:     uzs(parseMinor(it.Discount.String())),
			Other:        uzs(parseMinor(it.Other.String())),
			Barcode:      strPtr(it.Barcode),
			IKPUCode:     strPtr(it.ProductCode),
			IKPUName:     strPtr(it.ProductName),
			Unit:         strPtr(it.PackageName),
			ConsignorTIN: nonZeroNum(it.ComitentTin),
		}
		item.MarkingCode = strPtr(labels[it.ID])
		r.Items = append(r.Items, item)
	}

	return r, nil
}

func refundLabel(isRefund int) string {
	if isRefund == 1 {
		return "Qaytarish"
	}

	return "Sotuv"
}

func cardTypeLabel(code *int) *string {
	if code == nil {
		return nil
	}

	return strPtr(cardTypeLabels[*code])
}

// nonZeroNum returns a pointer to n's string form, or nil when empty or "0".
func nonZeroNum(n json.Number) *string {
	s := strings.TrimSpace(n.String())
	if s == "" || s == "0" {
		return nil
	}

	return &s
}

// parseSeq turns an int-ish receipt sequence string into a pointer.
func parseSeq(s string) *int {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if n, err := strconv.Atoi(s); err == nil {
		return &n
	}

	return nil
}

// strPtr trims s and returns a pointer, or nil when empty.
func strPtr(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	return &s
}
