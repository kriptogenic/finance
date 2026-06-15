// Package parser turns a raw bank-notification message into a structured
// Record. It is pure (no I/O) and fixture-tested against the real samples in
// REQUIREMENTS.md §4.1. It treats all input as untrusted: it never panics on an
// unexpected format — instead it returns a Record with Parsed=false carrying the
// raw text, so the caller can forward it to the operator rather than drop it
// (§4.2, §12).
package parser

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"
)

// Direction is the money-movement sign, taken from the ➖ / ➕ marker only —
// never from the descriptive type word, which is reused across transaction
// kinds (§4.2).
type Direction int

const (
	// Debit is an outgoing leg (➖): money leaves the card's account.
	Debit Direction = iota
	// Credit is an incoming leg (➕): money enters the card's account.
	Credit
)

func (d Direction) String() string {
	switch d {
	case Debit:
		return "debit"
	case Credit:
		return "credit"
	default:
		return fmt.Sprintf("Direction(%d)", int(d))
	}
}

// Record is one parsed notification. Money is always int64 minor units (×100);
// never float (§12). When Parsed is false, only RawText/ChatID/MessageID are
// meaningful — the message failed to parse and must be forwarded to the operator,
// not posted to the app (§4.2, §7).
type Record struct {
	ChatID    int64 // source chat; with MessageID forms the idempotency key
	MessageID int   // source Telegram message id

	Direction Direction // ➖/➕ only
	TypeWord  string    // Оплата/Операция/Пополнение — descriptive, not an enum

	Amount       int64     // minor units, e.g. 697.945,26 → 69794526
	Merchant     string    // raw, truncated, lossy — store, never key on it
	CardType     string    // e.g. HUMOCARD
	CardLast4    string    // e.g. 4853
	Time         time.Time // 🕓 minute-precision, in the configured location
	BalanceAfter int64     // 💰 minor units — reconciliation only (§8)

	Parsed  bool   // false → fail-loud raw passthrough
	RawText string // original message text, always retained
}

// ExternalID is the per-message idempotency key tg:<chat>:<msg> (§7). Transfer
// legs that pair get a different, shared key built in the pairing stage.
func (r Record) ExternalID() string {
	return fmt.Sprintf("tg:%d:%d", r.ChatID, r.MessageID)
}

// dateLayout is the 🕓 field layout: minute precision, no seconds (§4.2).
const dateLayout = "15:04 02.01.2006"

// vs is the optional Unicode variation selector (U+FE0F) that may follow a
// marker emoji; senders include it inconsistently, so we always allow it (§4.2).
const vs = `\x{fe0f}?`

// ws matches inter-field whitespace. It is deliberately broader than \s: bank
// notifications routinely use non-breaking (U+00A0) and narrow no-break
// (U+202F) spaces, which \s does not match, so \p{Zs} is included alongside it.
// \s still covers the newlines that separate the one-field-per-line format.
const ws = `[\s\p{Zs}]*`

// re anchors on the emoji markers, not field contents, and tolerates arbitrary
// whitespace (including newlines, via (?s) + \s*) between fields, so the same
// expression parses both the single-line and one-field-per-line formats (§4.2).
//
// Groups: sign, amount, merchant, cardType, cardLast4, datetime, balance.
var re = regexp.MustCompile(`(?s)` +
	`([➖➕])` + vs + ws + // direction sign marker (➖ debit / ➕ credit)
	`([0-9.,]+)` + ws + `UZS` + // amount
	ws + emoji("📍") + ws + `(.*?)` + // merchant (lazy, up to the card marker)
	ws + emoji("💳") + ws + `([A-Za-z]+)` + ws + `\*` + ws + `([0-9]+)` + // card type + last4
	ws + emoji("🕓") + ws + `([0-9:.]+` + ws + `[0-9.]+)` + // datetime (time<sep>date)
	ws + emoji("💰") + ws + `([0-9.,]+)` + ws + `UZS`) // balance after

// emoji builds a marker pattern that matches the literal emoji followed by an
// optional variation selector.
func emoji(e string) string { return regexp.QuoteMeta(e) + vs }

// typeWordRe captures the descriptive word after the leading 💸/🎉 (best effort;
// not used for any logic, only retained on the Record).
var typeWordRe = regexp.MustCompile(`^\s*(?:` + emoji("💸") + `|` + emoji("🎉") + `)?\s*([\p{L}]+)`)

// Parser turns raw message text into Records. It is pure and safe to share: the
// only state is the location used to interpret the 🕓 field (§4.2). Construct it
// with New.
type Parser struct {
	loc *time.Location
}

// New returns a Parser that interprets notification times in loc (typically
// Asia/Tashkent, §4.2). A nil loc falls back to UTC rather than panicking.
func New(loc *time.Location) *Parser {
	if loc == nil {
		loc = time.UTC
	}
	return &Parser{loc: loc}
}

// Parse turns raw message text into a Record. chatID and messageID are the
// source-message coordinates used for the idempotency key. Parse never returns
// an error: a message it cannot fully understand comes back with Parsed=false
// and the raw text retained, so the caller forwards rather than drops it (§4.2).
func (p *Parser) Parse(chatID int64, messageID int, raw string) Record {
	rec := Record{
		ChatID:    chatID,
		MessageID: messageID,
		RawText:   raw,
		Parsed:    false,
	}

	m := re.FindStringSubmatch(raw)
	if m == nil {
		return rec
	}

	sign := m[1]
	amount, err := parseMoney(m[2])
	if err != nil {
		return rec
	}
	merchant := strings.TrimSpace(m[3])
	cardType := strings.TrimSpace(m[4])
	cardLast4 := strings.TrimSpace(m[5])
	when, err := time.ParseInLocation(dateLayout, normalizeSpaces(m[6]), p.loc)
	if err != nil {
		return rec
	}
	balance, err := parseMoney(m[7])
	if err != nil {
		return rec
	}

	switch sign {
	case "➖":
		rec.Direction = Debit
	case "➕":
		rec.Direction = Credit
	default:
		return rec
	}

	rec.Amount = amount
	rec.Merchant = merchant
	rec.CardType = cardType
	rec.CardLast4 = cardLast4
	rec.Time = when
	rec.BalanceAfter = balance
	rec.TypeWord = typeWord(raw)
	rec.Parsed = true

	return rec
}

// normalizeSpaces trims the string and collapses any Unicode space separator
// (NBSP, narrow NBSP, …) to an ASCII space, so the fixed datetime layout —
// which expects a plain space between time and date — parses regardless of
// which space byte the bank used.
func normalizeSpaces(s string) string {
	s = strings.TrimSpace(s)
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return ' '
		}
		return r
	}, s)
}

// typeWord extracts the descriptive word (Оплата/Операция/Пополнение). Best
// effort and informational only — direction comes from the sign, never this.
func typeWord(raw string) string {
	if m := typeWordRe.FindStringSubmatch(raw); m != nil {
		return strings.TrimSpace(m[1])
	}
	return ""
}
