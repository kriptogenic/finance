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

type Poster interface {
	Post(ctx context.Context, p pairing.Posting) error
}

// BalancePoster delivers card balance snapshots for reconciliation.
type BalancePoster interface {
	PostBalances(ctx context.Context, balances []parser.CardBalance, reportedAt time.Time) error
}

type App struct {
	parser   *parser.Parser
	buffer   *pairing.Buffer
	poster   Poster
	balances BalancePoster
	store    *store.Store
	recon    *recon.Reconciler
	interval time.Duration
	log      *zap.Logger

	now func() time.Time

	watermark int
}

func New(p *parser.Parser, buf *pairing.Buffer, poster Poster, balances BalancePoster, st *store.Store, rc *recon.Reconciler, interval time.Duration, log *zap.Logger) *App {
	state := st.State()
	buf.Restore(state.Pending)
	return &App{
		parser:    p,
		buffer:    buf,
		poster:    poster,
		balances:  balances,
		store:     st,
		recon:     rc,
		interval:  interval,
		log:       log,
		now:       time.Now,
		watermark: state.Watermark,
	}
}

func (a *App) Run(ctx context.Context, f telegram.Fetcher) error {
	a.log.Info("starting poll loop",
		zap.Int("watermark", a.watermark),
		zap.Int("pending_legs", len(a.buffer.Pending())),
		zap.Duration("interval", a.interval),
	)

	if err := a.runCycle(ctx, f); err != nil {
		return err
	}
	ticker := time.NewTicker(a.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			a.log.Info("shutdown: persisting state", zap.Int("watermark", a.watermark))

			if err := a.persist(); err != nil {
				a.log.Error("final state persist failed", zap.Error(err))
			}
			return ctx.Err()
		case <-ticker.C:
			if err := a.runCycle(ctx, f); err != nil {
				return err
			}
		}
	}
}

func (a *App) runCycle(ctx context.Context, f telegram.Fetcher) error {
	if err := a.cycle(ctx, f); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		a.log.Error("poll cycle failed, will retry", zap.Error(err))
	}
	return nil
}

func (a *App) cycle(ctx context.Context, f telegram.Fetcher) error {
	msgs, err := f.FetchNewer(ctx, a.watermark)
	if err != nil {
		return err
	}
	chatID := f.ChatID()
	for _, m := range msgs {
		if err := a.process(ctx, chatID, m); err != nil {

			return err
		}
	}
	return a.flush(ctx)
}

func (a *App) process(ctx context.Context, chatID int64, m telegram.Message) error {
	// A balance-snapshot message reports current per-card balances; route it to
	// reconciliation instead of the transaction pipeline.
	if cards, ok := parser.ParseBalances(m.Text); ok {
		if err := a.balances.PostBalances(ctx, cards, m.Date); err != nil {
			return err
		}
		a.log.Info("balance snapshot ingested", zap.Int("message_id", m.ID), zap.Int("cards", len(cards)))
		return a.commit(m.ID)
	}

	rec := a.parser.Parse(chatID, m.ID, m.Text)
	a.recon.Check(rec)

	if !rec.Parsed {

		a.log.Error("unparseable bank message — not posted, needs operator review",
			zap.Int("message_id", m.ID),
			zap.String("raw_text", rec.RawText),
		)
		return a.commit(m.ID)
	}

	// The transaction message also reports the card's post-transaction balance;
	// feed it to reconciliation so gaps surface even without a snapshot message.
	if bal, ok := rec.Balance(); ok {
		if err := a.balances.PostBalances(ctx, []parser.CardBalance{bal}, rec.Time); err != nil {
			return err
		}
	}

	posting := a.buffer.Add(rec, a.now())
	if posting != nil {

		if err := a.deliver(ctx, *posting); err != nil {
			return err
		}
	}
	return a.commit(m.ID)
}

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

func (a *App) commit(messageID int) error {
	a.watermark = messageID
	return a.persist()
}

func (a *App) persist() error {
	return a.store.Save(a.watermark, a.buffer.Pending())
}
