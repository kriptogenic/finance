package pairing

import "finance/lookout/internal/parser"

var defaultTags = []string{"humo"}

// Standalone maps a single parsed bank record to a one-sided posting: an expense
// for a debit, income for a credit. Transfer pairing now happens server-side in
// core, which is the only place that sees legs from every source (this bot, the
// Android SMS/push app), so lookout just forwards each leg as it arrives.
func Standalone(rec parser.Record) Posting {
	p := Posting{
		ExternalID: rec.ExternalID(),
		Merchant:   rec.Merchant,
		Amount:     rec.Amount,
		Date:       rec.Time,
		Tags:       defaultTags,
	}
	if rec.Direction == parser.Debit {
		p.Type = "expense"
		p.FromCardLast4 = rec.CardLast4
	} else {
		p.Type = "income"
		p.ToCardLast4 = rec.CardLast4
	}

	return p
}
