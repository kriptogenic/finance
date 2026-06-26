package receipts

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"

	"finance/internal/entities"
)

// placemarkRe pulls merchant coordinates out of the inline Yandex Maps script.
var placemarkRe = regexp.MustCompile(`new ymaps\.Placemark\(\[([0-9.]+),\s*([0-9.]+)\]`)

// receivedAtRe matches a date like "01.02.2024, 13:45" inside an <i> tag.
var receivedAtRe = regexp.MustCompile(`(\d{2}\.\d{2}\.\d{4}),?\s+(\d{2}:\d{2})`)

// parseAmount turns a displayed amount like "16,480.00" (comma thousands, dot
// decimal, always 2 dp) into integer minor units (tiyin) without floats.
func parseAmount(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, ".", "")

	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse amount %q: %w", s, err)
	}

	return v, nil
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
		if ts, err := time.Parse("20060102150405", c); err == nil {
			receivedAt = &ts
		}
	}

	return terminalID, seq, fiscalSign, receivedAt
}

// ParseHTML scrapes a fiscal receipt page into a Receipt. It follows the
// selectors documented in QR_RECEIPT.md; they may need tuning against live
// ofd.soliq.uz markup.
func ParseHTML(html string) (entities.Receipt, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return entities.Receipt{}, fmt.Errorf("parse html: %w", err)
	}

	var r entities.Receipt

	r.ReceiptType = strPtr(doc.Find("h3").First().Text())
	r.MerchantName = strPtr(doc.Find(`h3[style*="font-weight: bold"]`).First().Text())

	// merchant tin: the first <i> made of digits; merchant address: the text
	// node sitting before it.
	doc.Find("i").EachWithBreak(func(_ int, i *goquery.Selection) bool {
		txt := strings.TrimSpace(i.Text())
		if isDigits(txt) {
			r.MerchantTIN = strPtr(txt)
			if prev := strings.TrimSpace(nodeTextBefore(i)); prev != "" {
				r.MerchantAddress = strPtr(prev)
			}

			return false
		}

		return true
	})

	r.ReceiptSeq = parseSeq(spanValue(doc, "Chek raqami"))
	r.DeviceName = strPtr(spanValue(doc, "Onlayn NKM nomi"))
	r.SerialNumber = strPtr(spanValue(doc, "SN"))

	if m := receivedAtRe.FindStringSubmatch(doc.Find("i").Text()); m != nil {
		if ts, err := time.Parse("02.01.2006 15:04", m[1]+" "+m[2]); err == nil {
			r.ReceivedAt = &ts
		}
	}

	r.PaidCash = rowAmount(doc, "Naqd pul")
	r.PaidCard = rowAmount(doc, "Bank kartalari")
	r.CardType = strPtr(rowText(doc, "Bank kartasi turi"))
	r.TotalAmount = rowAmount(doc, "Jami to'lov")
	r.TotalVAT = rowAmount(doc, "Umumiy QQS qiymati")

	if m := placemarkRe.FindStringSubmatch(html); m != nil {
		r.MerchantLat = strPtr(m[1])
		r.MerchantLng = strPtr(m[2])
	}

	r.Items = parseItems(doc)

	return r, nil
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

func parseItems(doc *goquery.Document) []entities.ReceiptItem {
	var items []entities.ReceiptItem

	doc.Find("tr.products-row").Each(func(_ int, prod *goquery.Selection) {
		group := prod.NextUntil("tr.products-row").AddSelection(prod)

		tds := prod.Find("td")
		item := entities.ReceiptItem{
			Name:     strings.TrimSpace(tds.Eq(0).Text()),
			Quantity: strings.TrimSpace(tds.Eq(1).Text()),
		}

		item.Price, _ = parseAmount(group.Find(".price-sum").First().Text())

		nds := group.Find(".nds-sum")
		item.VATAmount, _ = parseAmount(nds.Eq(0).Text())
		item.VATRate = parseVATRate(nds.Eq(1).Text())

		if cb := rowValueIn(group, "Chegirma/Boshqa"); cb != "" {
			left, right, _ := strings.Cut(cb, "/")
			item.Discount, _ = parseAmount(left)
			item.Other, _ = parseAmount(right)
		}

		item.Barcode = strPtr(rowValueIn(group, "Shtrix kodi"))
		item.IKPUCode = strPtr(rowValueIn(group, "MXIK kodi"))
		item.IKPUName = strPtr(rowValueIn(group, "MXIK nomi"))
		item.Unit = strPtr(rowValueIn(group, "O'lchov birligi"))
		item.MarkingCode = strPtr(rowValueIn(group, "Markirovka kodi"))
		item.ConsignorTIN = strPtr(rowValueIn(group, "Komitent STIR/JSHSHIR"))

		items = append(items, item)
	})

	return items
}

// parseVATRate strips a trailing % and parses the integer percent.
func parseVATRate(s string) int {
	s = strings.TrimSpace(strings.ReplaceAll(s, "%", ""))
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}

	return 0
}

// spanValue returns the <b> text inside a `span.left` whose text contains label.
func spanValue(doc *goquery.Document, label string) string {
	var val string
	doc.Find("span.left").EachWithBreak(func(_ int, span *goquery.Selection) bool {
		if strings.Contains(span.Text(), label) {
			val = strings.TrimSpace(span.Find("b").First().Text())

			return false
		}

		return true
	})

	return val
}

// rowText returns the value cell of the first <tr> whose first <td> contains label.
func rowText(doc *goquery.Document, label string) string {
	return rowValueIn(doc.Find("tr"), label)
}

// rowAmount is rowText parsed as a minor-unit amount.
func rowAmount(doc *goquery.Document, label string) int64 {
	v, _ := parseAmount(rowText(doc, label))

	return v
}

// rowValueIn finds, among the given rows, the first <tr> whose first <td>
// contains label and returns the trailing value (last <td>, or the remainder of
// a single-cell row).
func rowValueIn(rows *goquery.Selection, label string) string {
	var val string
	rows.EachWithBreak(func(_ int, tr *goquery.Selection) bool {
		tds := tr.Find("td")
		if tds.Length() == 0 {
			return true
		}
		first := strings.TrimSpace(tds.First().Text())
		if !strings.Contains(first, label) {
			return true
		}
		if tds.Length() >= 2 {
			val = strings.TrimSpace(tds.Last().Text())
		} else {
			val = strings.TrimSpace(strings.TrimPrefix(first, label))
		}

		return false
	})

	return val
}

// nodeTextBefore returns the nearest non-empty text node preceding sel.
func nodeTextBefore(sel *goquery.Selection) string {
	for node := sel.Nodes[0].PrevSibling; node != nil; node = node.PrevSibling {
		if node.Type == html.TextNode {
			if t := strings.TrimSpace(node.Data); t != "" {
				return t
			}
		}
	}

	return ""
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}

	return true
}

// strPtr trims s and returns a pointer, or nil when empty.
func strPtr(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	return &s
}
