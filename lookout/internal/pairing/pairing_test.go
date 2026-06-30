package pairing

import (
	"testing"
	"time"

	"finance/lookout/internal/parser"
)

func leg(id int, dir parser.Direction, amount int64, card string, txTime time.Time) parser.Record {
	return parser.Record{
		ChatID:    1,
		MessageID: id,
		Direction: dir,
		Amount:    amount,
		CardLast4: card,
		Time:      txTime,
		Merchant:  "TBC HUMO P2P>TASHKEN",
		Parsed:    true,
	}
}

func TestStandalone_DebitIsExpense(t *testing.T) {
	t0 := time.Date(2026, 6, 14, 9, 39, 0, 0, time.UTC)
	p := Standalone(leg(20, parser.Debit, 50000000, "8400", t0))

	if p.Type != "expense" {
		t.Errorf("type: got %q want expense", p.Type)
	}
	if p.FromCardLast4 != "8400" || p.ToCardLast4 != "" {
		t.Errorf("expense must set only from card: %+v", p)
	}
	if p.ExternalID != "tg:1:20" {
		t.Errorf("external id: got %q want tg:1:20", p.ExternalID)
	}
	if p.Amount != 50000000 || p.Merchant != "TBC HUMO P2P>TASHKEN" {
		t.Errorf("bad amount/merchant: %+v", p)
	}
}

func TestStandalone_CreditIsIncome(t *testing.T) {
	t0 := time.Date(2026, 6, 14, 9, 39, 0, 0, time.UTC)
	p := Standalone(leg(21, parser.Credit, 50000000, "4853", t0))

	if p.Type != "income" {
		t.Errorf("type: got %q want income", p.Type)
	}
	if p.ToCardLast4 != "4853" || p.FromCardLast4 != "" {
		t.Errorf("income must set only to card: %+v", p)
	}
	if p.ExternalID != "tg:1:21" {
		t.Errorf("external id: got %q want tg:1:21", p.ExternalID)
	}
}
