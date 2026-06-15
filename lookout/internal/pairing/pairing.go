// Package pairing buffers parsed debit/credit legs and collapses the two legs of
// an internal own-card transfer into a single transfer Posting (§5.1). It is the
// only stage that needs the temporal message stream; keeping it here prevents a
// phantom expense+income pair (which would double-count and break net worth)
// from reaching the ledger.
//
// The buffer is a deterministic state machine driven by an explicit `now` passed
// into Add and Tick — it owns no real timers and does no I/O, so it is fully
// unit-testable and its pending legs can be snapshotted for persistence across
// restarts (§8). The orchestrator calls Tick periodically to flush legs whose
// hold has expired.
package pairing

import (
	"fmt"
	"time"

	"finance/lookout/internal/parser"
)

// transferTag is added to every Posting so ingested rows are attributable to the
// bot / the Humo feed.
var defaultTags = []string{"humo"}

// PendingLeg is a buffered leg awaiting its transfer mate. ArrivalAt is the wall
// clock when it entered the buffer (drives the hold timeout); the leg's own 🕓
// time lives on Record and drives pair-window matching. PendingLeg is exported
// and JSON-serialisable so the store can persist it across restarts (§8).
type PendingLeg struct {
	Record    parser.Record `json:"record"`
	ArrivalAt time.Time     `json:"arrival_at"`
}

// Buffer holds unmatched legs and pairs them. It is not safe for concurrent use;
// the orchestrator drives it from a single goroutine.
type Buffer struct {
	// pairWindow is the max gap between the two legs' 🕓 transaction times for
	// them to pair (≈120s) (§5.1).
	pairWindow time.Duration
	// hold is how long an unmatched leg waits before being flushed as a
	// standalone expense/income (≈5m, must exceed poll interval + skew so a leg
	// never times out before its mate is polled) (§5.1).
	hold time.Duration

	pending []PendingLeg
}

// New returns a Buffer with the two pairing timers (§5.1, §9).
func New(pairWindow, hold time.Duration) *Buffer {
	return &Buffer{pairWindow: pairWindow, hold: hold}
}

// Add ingests one parsed leg at wall-clock time now. If a matching counterpart
// is already buffered (opposite sign, exactly equal amount, different own card,
// |🕓Δ| ≤ pairWindow) the two collapse into a single transfer Posting and the
// mate is removed from the buffer; the new leg is not buffered. Otherwise the
// leg is held and Add returns nil. Matching the buffer (not arrival order) makes
// out-of-order arrival irrelevant (§5.1).
//
// Add must be called only with a successfully-parsed debit/credit record; an
// unparseable message is the operator's to handle and never reaches pairing
// (§7).
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

// Tick flushes every leg whose hold has expired (ArrivalAt+hold ≤ now) as a
// standalone expense (debit) or income (credit) Posting, removing it from the
// buffer. It is the safeguard that guarantees a leg outlives poll latency before
// being posted alone (§5.1). Returns the flushed postings in arrival order.
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

// findMate returns the index of a buffered leg that forms a transfer with rec,
// preferring the closest in 🕓 time when several qualify.
func (b *Buffer) findMate(rec parser.Record) (int, bool) {
	best := -1
	var bestGap time.Duration
	for i, leg := range b.pending {
		o := leg.Record
		if o.Direction == rec.Direction { // need opposite signs
			continue
		}
		if o.Amount != rec.Amount { // exact match: fees arrive separately (§5.1)
			continue
		}
		if o.CardLast4 == rec.CardLast4 { // must be a different own card
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

// transfer builds the single transfer Posting from a matched debit/credit pair.
// from = the debit's card, to = the credit's card; no merchant/category (§5.1).
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
		Date:            debit.Time, // money-leaves time
		TransferGroupID: id,
		Tags:            defaultTags,
	}
}

// standalone maps a lone leg to an expense (debit) or income (credit) Posting
// (§5). The merchant is forwarded raw for the app to route to a category.
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

// Pending returns a copy of the buffered legs for persistence (§8).
func (b *Buffer) Pending() []PendingLeg {
	out := make([]PendingLeg, len(b.pending))
	copy(out, b.pending)
	return out
}

// Restore replaces the buffer contents with previously-persisted legs on startup
// (§8), so a restart does not lose half a transfer.
func (b *Buffer) Restore(legs []PendingLeg) {
	b.pending = make([]PendingLeg, len(legs))
	copy(b.pending, legs)
}

// transferExternalID is the deterministic idempotency key for a collapsed
// transfer: tg:transfer:<id_lo>-<id_hi>, order-independent so either arrival
// order yields the same key and the app dedupes (§7).
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
