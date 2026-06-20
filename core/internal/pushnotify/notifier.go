// Package pushnotify broadcasts the uncategorized-transaction count to the
// user's registered devices as a Web Push, so the PWA app-icon badge updates
// even while the app is closed.
package pushnotify

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"finance/config"
	"finance/internal/entities"
	categoryrepository "finance/internal/repositories/category_repository"
	pushsubscriptionrepository "finance/internal/repositories/push_subscription_repository"
	transactionrepository "finance/internal/repositories/transaction_repository"
	"finance/pkg/webpush"
)

// broadcastTimeout bounds the whole fan-out, which runs detached from the request.
const broadcastTimeout = 20 * time.Second

// Notifier reacts to ingest events by refreshing the badge on every device.
type Notifier interface {
	// OnIngestedCategory fires after an ingest created a transaction in catID.
	// When that category is an Uncategorized bucket it pushes the new count.
	// It returns immediately; delivery happens in the background.
	OnIngestedCategory(catID uuid.UUID)
}

// payload is the JSON the service worker receives on a push event.
type payload struct {
	Type  string `json:"type"`
	Count int    `json:"count"`
}

type notifier struct {
	subs   pushsubscriptionrepository.Repository
	txns   transactionrepository.Repository
	cats   categoryrepository.Repository
	sender *webpush.Sender
	logger *zap.Logger
}

// New returns a Notifier. With no VAPID keys configured it is a no-op.
func New(
	cfg *config.Push,
	subs pushsubscriptionrepository.Repository,
	txns transactionrepository.Repository,
	cats categoryrepository.Repository,
	logger *zap.Logger,
) Notifier {
	if cfg.VAPIDPublic == "" || cfg.VAPIDPrivate == "" {
		logger.Info("push notifications disabled: VAPID keys not set")

		return disabled{}
	}

	return &notifier{
		subs:   subs,
		txns:   txns,
		cats:   cats,
		sender: webpush.New(cfg.VAPIDPublic, cfg.VAPIDPrivate, cfg.Subscriber),
		logger: logger,
	}
}

func (n *notifier) OnIngestedCategory(catID uuid.UUID) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), broadcastTimeout)
		defer cancel()

		if err := n.run(ctx, catID); err != nil {
			n.logger.Error("push badge broadcast", zap.Error(err))
		}
	}()
}

func (n *notifier) run(ctx context.Context, catID uuid.UUID) error {
	// A rule-matched category doesn't change the uncategorized badge — skip.
	cat, err := n.cats.Get(ctx, catID)
	if err != nil {
		return err
	}
	if !isUncategorized(cat) {
		return nil
	}

	count, err := n.txns.CountUncategorized(ctx)
	if err != nil {
		return err
	}

	subs, err := n.subs.List(ctx)
	if err != nil {
		return err
	}
	if len(subs) == 0 {
		return nil
	}

	body, err := json.Marshal(payload{Type: "uncategorized", Count: count})
	if err != nil {
		return err
	}

	for _, sub := range subs {
		err = n.sender.Send(ctx, webpush.Subscription{
			Endpoint: sub.Endpoint,
			P256dh:   sub.P256dh,
			Auth:     sub.Auth,
		}, body)
		switch {
		case errors.Is(err, webpush.ErrGone):
			if delErr := n.subs.Delete(ctx, sub.Endpoint); delErr != nil {
				n.logger.Warn("prune dead push subscription", zap.Error(delErr))
			}
		case err != nil:
			n.logger.Warn("push delivery failed", zap.Error(err))
		}
	}

	return nil
}

func isUncategorized(cat *entities.Category) bool {
	if cat.SystemKey == nil {
		return false
	}

	return *cat.SystemKey == "uncategorized_expense" || *cat.SystemKey == "uncategorized_income"
}

type disabled struct{}

func (disabled) OnIngestedCategory(uuid.UUID) {}
