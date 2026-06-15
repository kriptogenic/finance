// Package app is the orchestrator: it wires the stages (parser → pairing →
// delivery, with store + recon) and owns the poll loop. It is the only place
// that advances the watermark, and it does so only after a leg is delivered
// (201/200) or safely buffered and persisted — so a crash re-processes rather
// than skips, and the app's idempotency makes re-processing harmless (§7, §8).
package app

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"

	"finance/lookout/internal/delivery"
	"finance/lookout/internal/pairing"
	"finance/lookout/internal/parser"
	"finance/lookout/internal/recon"
	"finance/lookout/internal/store"
	"finance/lookout/internal/telegram"
)

// Poster delivers one Posting, retrying transient failures internally and
// returning a delivery.ErrPermanent for 400/401 (matched by the orchestrator).
type Poster interface {
	Post(ctx context.Context, p pairing.Posting) error
}

// App holds the wired pipeline and mutable run state. The poll loop is
// single-goroutine, so the buffer/watermark need no locking.
type App struct {
	parser   *parser.Parser
	buffer   *pairing.Buffer
	poster   Poster
	store    *store.Store
	recon    *recon.Reconciler
	interval time.Duration
	log      *zap.Logger

	// now is the clock for buffer hold/pair timing; overridable in tests.
	now func() time.Time

	watermark int
}

// New builds the orchestrator and restores persisted state (watermark + pending
// transfer legs) so a restart resumes exactly where it left off (§8).
func New(p *parser.Parser, buf *pairing.Buffer, poster Poster, st *store.Store, rc *recon.Reconciler, interval time.Duration, log *zap.Logger) *App {
	state := st.State()
	buf.Restore(state.Pending)
	return &App{
		parser:    p,
		buffer:    buf,
		poster:    poster,
		store:     st,
		recon:     rc,
		interval:  interval,
		log:       log,
		now:       time.Now,
		watermark: state.Watermark,
	}
}

// Run polls f every interval until ctx is cancelled. It is the function handed to
// telegram.Source.Run, so it executes inside the live Telegram connection.
func (a *App) Run(ctx context.Context, f telegram.Fetcher) error {
	a.log.Info("starting poll loop",
		zap.Int("watermark", a.watermark),
		zap.Int("pending_legs", len(a.buffer.Pending())),
		zap.Duration("interval", a.interval),
	)

	// Run one cycle immediately, then on the ticker.
	if err := a.cycle(ctx, f); err != nil {
		return err
	}
	ticker := time.NewTicker(a.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			a.log.Info("shutdown: persisting state", zap.Int("watermark", a.watermark))
			// Best-effort final persist of the buffer (pending legs survive).
			if err := a.persist(); err != nil {
				a.log.Error("final state persist failed", zap.Error(err))
			}
			return ctx.Err()
		case <-ticker.C:
			if err := a.cycle(ctx, f); err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return err
				}
				// A transient cycle error (e.g. history fetch hiccup) should not
				// kill the bot; log and retry on the next tick.
				a.log.Error("poll cycle failed, will retry", zap.Error(err))
			}
		}
	}
}

// cycle fetches new messages, processes them in order, then flushes any
// transfer legs whose hold has expired.
func (a *App) cycle(ctx context.Context, f telegram.Fetcher) error {
	msgs, err := f.FetchNewer(ctx, a.watermark)
	if err != nil {
		return err
	}
	chatID := f.ChatID()
	for _, m := range msgs {
		if err := a.process(ctx, chatID, m); err != nil {
			// Stop the batch without advancing past the failed message; the next
			// cycle retries from the same watermark (idempotent on the app side).
			return err
		}
	}
	return a.flush(ctx)
}

// process handles one message: parse, reconcile, pair, deliver any formed
// transfer, then commit the watermark + buffer together.
func (a *App) process(ctx context.Context, chatID int64, m telegram.Message) error {
	rec := a.parser.Parse(chatID, m.ID, m.Text)
	a.recon.Check(rec) // logs a gap if the balance is discontinuous (§8)

	if !rec.Parsed {
		// Fail loud: never drop. The app feed is strongly-typed, so an
		// unparseable message is logged for the operator and NOT posted (§4.2,§7).
		a.log.Error("unparseable bank message — not posted, needs operator review",
			zap.Int("message_id", m.ID),
			zap.String("raw_text", rec.RawText),
		)
		return a.commit(m.ID)
	}

	posting := a.buffer.Add(rec, a.now())
	if posting != nil {
		// A transfer formed (this leg + a buffered mate). Deliver before
		// committing so a failure replays rather than loses the mate.
		if err := a.deliver(ctx, *posting); err != nil {
			return err
		}
	}
	return a.commit(m.ID)
}

// flush delivers transfer legs whose hold expired, as standalone expense/income.
// Their message IDs are already at/below the watermark, so only the buffer
// changes; persist after each successful delivery so a crash replays undelivered
// legs (which then dedupe).
func (a *App) flush(ctx context.Context) error {
	for _, p := range a.buffer.Tick(a.now()) {
		if err := a.deliver(ctx, p); err != nil {
			return err
		}
		if err := a.persist(); err != nil {
			return err
		}
	}
	return nil
}

// deliver posts a single Posting. A permanent error (unknown card / bad token)
// is logged loudly and surfaced so the cycle stops without advancing — once the
// operator fixes the cause (e.g. sets the card's account in the app) the next
// cycle succeeds and dedupes.
func (a *App) deliver(ctx context.Context, p pairing.Posting) error {
	err := a.poster.Post(ctx, p)
	if err == nil {
		a.log.Info("ingested",
			zap.String("external_id", p.ExternalID),
			zap.String("type", p.Type),
			zap.Int64("amount", p.Amount),
		)
		return nil
	}
	if errors.Is(err, delivery.ErrPermanent) {
		a.log.Error("permanent ingest rejection — operator must resolve (e.g. unknown card)",
			zap.String("external_id", p.ExternalID),
			zap.String("type", p.Type),
			zap.Error(err),
		)
	}
	return err
}

// commit advances the watermark and atomically persists it with the buffer (§8).
func (a *App) commit(messageID int) error {
	a.watermark = messageID
	return a.persist()
}

func (a *App) persist() error {
	return a.store.Save(a.watermark, a.buffer.Pending())
}
