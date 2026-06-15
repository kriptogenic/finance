package pairing

import (
	"testing"
	"time"

	"finance/lookout/internal/parser"
)

const (
	pairWindow = 2 * time.Minute
	hold       = 5 * time.Minute
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

func TestPair_InOrder(t *testing.T) {
	b := New(pairWindow, hold)
	t0 := time.Date(2026, 6, 14, 9, 36, 0, 0, time.UTC)

	if got := b.Add(leg(10, parser.Debit, 100000000, "4853", t0), t0); got != nil {
		t.Fatalf("first leg should be held, got posting %+v", got)
	}
	got := b.Add(leg(11, parser.Credit, 100000000, "8400", t0), t0.Add(20*time.Second))
	if got == nil {
		t.Fatalf("matching credit should produce a transfer")
	}
	if got.Type != "transfer" {
		t.Errorf("type: got %q want transfer", got.Type)
	}
	if got.FromCardLast4 != "4853" || got.ToCardLast4 != "8400" {
		t.Errorf("cards: from %q to %q", got.FromCardLast4, got.ToCardLast4)
	}
	if got.Amount != 100000000 {
		t.Errorf("amount: got %d", got.Amount)
	}
	if got.ExternalID != "tg:transfer:10-11" {
		t.Errorf("external id: got %q want tg:transfer:10-11", got.ExternalID)
	}
	if got.Merchant != "" {
		t.Errorf("transfer must carry no merchant/category, got %q", got.Merchant)
	}
	if len(b.Pending()) != 0 {
		t.Errorf("buffer should be empty after pairing, has %d", len(b.Pending()))
	}
}

func TestPair_OutOfOrder(t *testing.T) {
	b := New(pairWindow, hold)
	t0 := time.Date(2026, 6, 14, 9, 36, 0, 0, time.UTC)

	b.Add(leg(11, parser.Credit, 100000000, "8400", t0), t0)
	got := b.Add(leg(10, parser.Debit, 100000000, "4853", t0.Add(time.Minute)), t0.Add(time.Minute))
	if got == nil {
		t.Fatalf("debit should pair with the earlier-buffered credit")
	}
	if got.FromCardLast4 != "4853" || got.ToCardLast4 != "8400" {
		t.Errorf("cards: from %q to %q (must follow sign, not arrival)", got.FromCardLast4, got.ToCardLast4)
	}
	if got.ExternalID != "tg:transfer:10-11" {
		t.Errorf("external id must be order-independent: got %q", got.ExternalID)
	}
}

func TestPair_NoMateTimeout(t *testing.T) {
	b := New(pairWindow, hold)
	t0 := time.Date(2026, 6, 14, 9, 39, 0, 0, time.UTC)

	b.Add(leg(20, parser.Debit, 50000000, "8400", t0), t0)

	if out := b.Tick(t0.Add(hold - time.Second)); len(out) != 0 {
		t.Fatalf("leg flushed too early: %+v", out)
	}
	out := b.Tick(t0.Add(hold))
	if len(out) != 1 {
		t.Fatalf("expected 1 flushed posting, got %d", len(out))
	}
	if out[0].Type != "expense" || out[0].FromCardLast4 != "8400" {
		t.Errorf("expected expense from 8400, got %+v", out[0])
	}
	if out[0].ExternalID != "tg:1:20" {
		t.Errorf("external id: got %q want tg:1:20", out[0].ExternalID)
	}
	if len(b.Pending()) != 0 {
		t.Errorf("buffer should be empty after flush")
	}
}

func TestPair_LoneCreditIsIncome(t *testing.T) {
	b := New(pairWindow, hold)
	t0 := time.Date(2026, 6, 14, 9, 39, 0, 0, time.UTC)
	b.Add(leg(21, parser.Credit, 50000000, "4853", t0), t0)
	out := b.Tick(t0.Add(hold))
	if len(out) != 1 || out[0].Type != "income" || out[0].ToCardLast4 != "4853" {
		t.Fatalf("expected income to 4853, got %+v", out)
	}
}

func TestPair_WindowGuardPreventsFalseMerge(t *testing.T) {
	b := New(pairWindow, hold)
	t0 := time.Date(2026, 6, 14, 9, 0, 0, 0, time.UTC)

	b.Add(leg(30, parser.Debit, 100000000, "4853", t0), t0)

	got := b.Add(leg(31, parser.Credit, 100000000, "8400", t0.Add(5*time.Minute)), t0.Add(10*time.Second))
	if got != nil {
		t.Fatalf("legs outside the pair window must not merge, got %+v", got)
	}
	if len(b.Pending()) != 2 {
		t.Fatalf("both legs should remain buffered, have %d", len(b.Pending()))
	}
}

func TestPair_SameCardDoesNotMerge(t *testing.T) {
	b := New(pairWindow, hold)
	t0 := time.Date(2026, 6, 14, 9, 0, 0, 0, time.UTC)
	b.Add(leg(40, parser.Debit, 100000000, "4853", t0), t0)
	if got := b.Add(leg(41, parser.Credit, 100000000, "4853", t0), t0); got != nil {
		t.Fatalf("same-card legs must not merge, got %+v", got)
	}
}

func TestPair_DifferentAmountDoesNotMerge(t *testing.T) {
	b := New(pairWindow, hold)
	t0 := time.Date(2026, 6, 14, 9, 0, 0, 0, time.UTC)
	b.Add(leg(50, parser.Debit, 100000000, "4853", t0), t0)
	if got := b.Add(leg(51, parser.Credit, 99999999, "8400", t0), t0); got != nil {
		t.Fatalf("unequal amounts must not merge, got %+v", got)
	}
}

func TestPair_SnapshotRestore(t *testing.T) {
	b := New(pairWindow, hold)
	t0 := time.Date(2026, 6, 14, 9, 36, 0, 0, time.UTC)
	b.Add(leg(60, parser.Debit, 100000000, "4853", t0), t0)

	snap := b.Pending()

	b2 := New(pairWindow, hold)
	b2.Restore(snap)

	got := b2.Add(leg(61, parser.Credit, 100000000, "8400", t0), t0.Add(30*time.Second))
	if got == nil || got.ExternalID != "tg:transfer:60-61" {
		t.Fatalf("restored leg should still pair, got %+v", got)
	}
}
