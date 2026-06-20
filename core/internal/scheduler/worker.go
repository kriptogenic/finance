package scheduler

import (
	"context"
	"time"

	"go.uber.org/fx"
	"go.uber.org/zap"
)

// tickInterval is how often the worker scans for due schedules. Schedules have
// day granularity, so hourly is ample and cheap.
const tickInterval = time.Hour

// Worker periodically materializes due schedules. It runs once at startup (to
// catch anything that came due while the process was down) and then on a ticker.
type Worker struct {
	materializer *Materializer
	logger       *zap.Logger

	cancel context.CancelFunc
	done   chan struct{}
}

func NewWorker(materializer *Materializer, logger *zap.Logger) *Worker {
	return &Worker{materializer: materializer, logger: logger}
}

func (w *Worker) start(context.Context) error {
	ctx, cancel := context.WithCancel(context.Background())
	w.cancel = cancel
	w.done = make(chan struct{})

	go w.loop(ctx)

	return nil
}

func (w *Worker) stop(context.Context) error {
	if w.cancel != nil {
		w.cancel()
		<-w.done
	}

	return nil
}

func (w *Worker) loop(ctx context.Context) {
	defer close(w.done)

	w.tick(ctx)

	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.tick(ctx)
		}
	}
}

func (w *Worker) tick(ctx context.Context) {
	fired, err := w.materializer.RunDue(ctx, time.Now().UTC())
	if err != nil {
		w.logger.Error("schedule tick", zap.Error(err))

		return
	}
	if fired > 0 {
		w.logger.Info("materialized scheduled transactions", zap.Int("count", fired))
	}
}

// Lifecycle wires the worker into the fx app: start a goroutine on boot, cancel
// it on shutdown (pattern mirrors app.dbLifecycle).
func Lifecycle(lc fx.Lifecycle, w *Worker) {
	lc.Append(fx.Hook{
		OnStart: w.start,
		OnStop:  w.stop,
	})
}
