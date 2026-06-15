package pairing

import (
	"fmt"
	"time"

	"finance/lookout/internal/parser"
)

var defaultTags = []string{"humo"}

type PendingLeg struct {
	Record    parser.Record `json:"record"`
	ArrivalAt time.Time     `json:"arrival_at"`
}

type Buffer struct {
	pairWindow time.Duration

	hold time.Duration

	pending []PendingLeg
}

func New(pairWindow, hold time.Duration) *Buffer {
	return &Buffer{pairWindow: pairWindow, hold: hold}
}

func (b *Buffer) Add(rec parser.Record, now time.Time) *Posting {
	if i, ok := b.findMate(rec); ok {
		mate := b.pending[i].Record
		b.remove(i)
		t := b.transfer(rec, mate)
		return &t
	}
	b.pending = append(b.pending, PendingLeg{Record: rec, ArrivalAt: now})
	return nil
}

func (b *Buffer) Tick(now time.Time) []Posting {
	var out []Posting
	kept := b.pending[:0]
	for _, leg := range b.pending {
		if !now.Before(leg.ArrivalAt.Add(b.hold)) {
			out = append(out, b.standalone(leg.Record))
			continue
		}
		kept = append(kept, leg)
	}
	b.pending = kept
	return out
}

func (b *Buffer) findMate(rec parser.Record) (int, bool) {
	best := -1
	var bestGap time.Duration
	for i, leg := range b.pending {
		o := leg.Record
		if o.Direction == rec.Direction {
			continue
		}
		if o.Amount != rec.Amount {
			continue
		}
		if o.CardLast4 == rec.CardLast4 {
			continue
		}
		gap := absDur(o.Time.Sub(rec.Time))
		if gap > b.pairWindow {
			continue
		}
		if best == -1 || gap < bestGap {
			best, bestGap = i, gap
		}
	}
	return best, best != -1
}

func (b *Buffer) transfer(a, c parser.Record) Posting {
	debit, credit := a, c
	if debit.Direction == parser.Credit {
		debit, credit = credit, debit
	}
	id := transferExternalID(debit.MessageID, credit.MessageID)
	return Posting{
		ExternalID:      id,
		Type:            "transfer",
		FromCardLast4:   debit.CardLast4,
		ToCardLast4:     credit.CardLast4,
		Amount:          debit.Amount,
		Date:            debit.Time,
		TransferGroupID: id,
		Tags:            defaultTags,
	}
}

func (b *Buffer) standalone(rec parser.Record) Posting {
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

func (b *Buffer) remove(i int) {
	b.pending = append(b.pending[:i], b.pending[i+1:]...)
}

func (b *Buffer) Pending() []PendingLeg {
	out := make([]PendingLeg, len(b.pending))
	copy(out, b.pending)
	return out
}

func (b *Buffer) Restore(legs []PendingLeg) {
	b.pending = make([]PendingLeg, len(legs))
	copy(b.pending, legs)
}

func transferExternalID(a, b int) string {
	lo, hi := a, b
	if lo > hi {
		lo, hi = hi, lo
	}
	return fmt.Sprintf("tg:transfer:%d-%d", lo, hi)
}

func absDur(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}
