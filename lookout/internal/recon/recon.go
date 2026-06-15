package recon

import (
	"go.uber.org/zap"

	"finance/lookout/internal/parser"
)

type Reconciler struct {
	last map[string]int64
	log  *zap.Logger
}

func New(log *zap.Logger) *Reconciler {
	return &Reconciler{last: make(map[string]int64), log: log}
}

type Gap struct {
	Card     string
	Expected int64
	Got      int64
	Delta    int64
}

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

func signedAmount(rec parser.Record) int64 {
	if rec.Direction == parser.Credit {
		return rec.Amount
	}
	return -rec.Amount
}
