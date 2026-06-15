package recon

import (
	"testing"

	"go.uber.org/zap"

	"finance/lookout/internal/parser"
)

func rec(card string, dir parser.Direction, amount, balance int64) parser.Record {
	return parser.Record{CardLast4: card, Direction: dir, Amount: amount, BalanceAfter: balance, Parsed: true}
}

func TestRecon_NoGapOnContiguous(t *testing.T) {
	r := New(zap.NewNop())
	// First message for the card: recorded, no gap.
	if _, ok := r.Check(rec("4853", parser.Debit, 5755000, 69794526)); ok {
		t.Fatal("first message must not be a gap")
	}
	// Next debit: 69794526 - 1000000 = 68794526.
	if _, ok := r.Check(rec("4853", parser.Debit, 1000000, 68794526)); ok {
		t.Fatal("contiguous debit must not be a gap")
	}
	// Credit: 68794526 + 520000 = 69314526.
	if _, ok := r.Check(rec("4853", parser.Credit, 520000, 69314526)); ok {
		t.Fatal("contiguous credit must not be a gap")
	}
}

func TestRecon_DetectsMissedMessage(t *testing.T) {
	r := New(zap.NewNop())
	r.Check(rec("4853", parser.Debit, 1000000, 5000000)) // establish balance 5,000,000
	// A message is missed; the next one we see jumps to a balance that the
	// stated amount cannot explain.
	gap, ok := r.Check(rec("4853", parser.Debit, 1000000, 2000000)) // expected 4,000,000
	if !ok {
		t.Fatal("expected a gap")
	}
	if gap.Expected != 4000000 || gap.Got != 2000000 || gap.Delta != -2000000 {
		t.Fatalf("gap fields wrong: %+v", gap)
	}
	// After a gap, the balance resyncs so the next contiguous message is clean.
	if _, ok := r.Check(rec("4853", parser.Debit, 500000, 1500000)); ok {
		t.Fatal("gap should not cascade once resynced")
	}
}

func TestRecon_PerCardIndependent(t *testing.T) {
	r := New(zap.NewNop())
	r.Check(rec("4853", parser.Debit, 1000000, 5000000))
	r.Check(rec("8400", parser.Debit, 1000000, 9000000))
	// Each card reconciles against its own last balance.
	if _, ok := r.Check(rec("4853", parser.Debit, 1000000, 4000000)); ok {
		t.Fatal("4853 contiguous, no gap")
	}
	if _, ok := r.Check(rec("8400", parser.Credit, 1000000, 10000000)); ok {
		t.Fatal("8400 contiguous, no gap")
	}
}

func TestRecon_IgnoresUnparsed(t *testing.T) {
	r := New(zap.NewNop())
	if _, ok := r.Check(parser.Record{Parsed: false}); ok {
		t.Fatal("unparsed record must not produce a gap")
	}
}
