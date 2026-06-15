// Package recon detects missed or dropped notifications by checking each card's
// running balance (§8): the 💰 balance_after of message N should equal that of
// N−1 plus the signed amount of N. A mismatch means a message was missed between
// the two — even with gap-free polling this catches silent drops. recon only
// reports gaps; backfill/alerting is the caller's choice.
package recon

import (
	"go.uber.org/zap"

	"finance/lookout/internal/parser"
)

// Reconciler tracks the last seen balance per card. It is not safe for
// concurrent use; the poll loop drives it from one goroutine.
type Reconciler struct {
	last map[string]int64 // card last4 → last balance_after (minor units)
	log  *zap.Logger
}

// New returns an empty Reconciler.
func New(log *zap.Logger) *Reconciler {
	return &Reconciler{last: make(map[string]int64), log: log}
}

// Gap describes a detected discontinuity in a card's balance.
type Gap struct {
	Card     string
	Expected int64 // last balance ± this message's signed amount
	Got      int64 // the message's actual balance_after
	Delta    int64 // Got − Expected (size of the unexplained jump)
}

// Check reconciles one parsed record against the card's previous balance. It
// returns the detected Gap and ok=true when there is a discontinuity; either way
// it advances the card's last balance to the message's actual balance_after so a
// single gap doesn't cascade into false positives on every later message.
//
// Unparsed records (no reliable balance) and the first message for a card are
// recorded without a gap.
func (r *Reconciler) Check(rec parser.Record) (Gap, bool) {
	if !rec.Parsed {
		return Gap{}, false
	}
	prev, seen := r.last[rec.CardLast4]
	r.last[rec.CardLast4] = rec.BalanceAfter
	if !seen {
		return Gap{}, false
	}

	expected := prev + signedAmount(rec)
	if expected == rec.BalanceAfter {
		return Gap{}, false
	}
	gap := Gap{
		Card:     rec.CardLast4,
		Expected: expected,
		Got:      rec.BalanceAfter,
		Delta:    rec.BalanceAfter - expected,
	}
	if r.log != nil {
		r.log.Warn("balance reconciliation gap — a message was likely missed",
			zap.String("card_last4", gap.Card),
			zap.Int64("expected_balance", gap.Expected),
			zap.Int64("actual_balance", gap.Got),
			zap.Int64("unexplained_delta", gap.Delta),
		)
	}
	return gap, true
}

// signedAmount is +amount for a credit (balance rises) and −amount for a debit
// (balance falls).
func signedAmount(rec parser.Record) int64 {
	if rec.Direction == parser.Credit {
		return rec.Amount
	}
	return -rec.Amount
}
