package parser

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"
)

type Direction int

const (
	Debit Direction = iota

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

type Record struct {
	ChatID    int64
	MessageID int

	Direction Direction
	TypeWord  string

	Amount       int64
	Merchant     string
	CardType     string
	CardLast4    string
	Time         time.Time
	BalanceAfter int64

	Parsed  bool
	RawText string
}

func (r Record) ExternalID() string {
	return fmt.Sprintf("tg:%d:%d", r.ChatID, r.MessageID)
}

const dateLayout = "15:04 02.01.2006"

// txCurrency is the currency every transaction notification is denominated in.
const txCurrency = "UZS"

const vs = `\x{fe0f}?`

const ws = `[\t\f\r\p{Zs}]*`

const nl = ws + `\n` + ws

var re = regexp.MustCompile(
	`([➖➕])` + vs + ws +
		`([0-9.,]+)` + ws + txCurrency +
		nl + emoji("📍") + ws + `(.*?)` +
		nl + emoji("💳") + ws + `([A-Za-z]+)` + ws + `\*` + ws + `([0-9]+)` +
		nl + emoji("🕓") + ws + `([0-9:.]+` + ws + `[0-9.]+)` +
		nl + emoji("💰") + ws + `([0-9.,]+)` + ws + txCurrency)

func emoji(e string) string { return regexp.QuoteMeta(e) + vs }

var typeWordRe = regexp.MustCompile(`^\s*(?:` + emoji("💸") + `|` + emoji("🎉") + `)?\s*([\p{L}]+)`)

type Parser struct {
	loc *time.Location
}

func New(loc *time.Location) *Parser {
	if loc == nil {
		loc = time.UTC
	}
	return &Parser{loc: loc}
}

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

func normalizeSpaces(s string) string {
	s = strings.TrimSpace(s)
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return ' '
		}
		return r
	}, s)
}

func typeWord(raw string) string {
	if m := typeWordRe.FindStringSubmatch(raw); m != nil {
		return strings.TrimSpace(m[1])
	}
	return ""
}
