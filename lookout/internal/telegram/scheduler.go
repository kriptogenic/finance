package telegram

import (
	"context"
	"math/rand/v2"
	"time"

	"github.com/gotd/td/tg"
	"go.uber.org/zap"
)

const balanceMessage = "💰 Баланс"

const (
	scheduleHour   = 8
	scheduleMinute = 0
)

type balanceScheduler struct {
	api         *tg.Client
	peer        tg.InputPeerClass
	loc         *time.Location
	sendOnStart bool
	log         *zap.Logger
	now         func() time.Time
}

func (b *balanceScheduler) run(ctx context.Context) {
	if b.sendOnStart {
		b.trySend(ctx)
	}

	for {
		now := b.localNow()
		next := nextRun(now, scheduleHour, scheduleMinute)
		wait := next.Sub(now)
		b.log.Info("balance reminder scheduled", zap.Time("next_run", next), zap.Duration("in", wait))

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			b.trySend(ctx)
		}
	}
}

func (b *balanceScheduler) trySend(ctx context.Context) {
	if err := b.send(ctx); err != nil {
		b.log.Error("balance reminder send failed", zap.Error(err))

		return
	}
	b.log.Info("balance reminder sent", zap.String("text", balanceMessage))
}

func (b *balanceScheduler) send(ctx context.Context) error {
	_, err := b.api.MessagesSendMessage(ctx, &tg.MessagesSendMessageRequest{
		Peer:     b.peer,
		Message:  balanceMessage,
		RandomID: rand.Int64(),
	})

	return err
}

func (b *balanceScheduler) localNow() time.Time {
	now := b.now()
	if b.loc != nil {
		return now.In(b.loc)
	}

	return now
}

func nextRun(now time.Time, hour, min int) time.Time {
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, min, 0, 0, now.Location())
	if !next.After(now) {
		next = next.AddDate(0, 0, 1)
	}

	return next
}
