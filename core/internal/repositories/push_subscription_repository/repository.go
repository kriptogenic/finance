package pushsubscriptionrepository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"finance/internal/entities"
	"finance/pkg/database"
)

type Repository interface {
	// Upsert stores a subscription, keyed by endpoint; re-subscribing refreshes its keys.
	Upsert(ctx context.Context, sub *entities.PushSubscription) error
	List(ctx context.Context) ([]entities.PushSubscription, error)
	Delete(ctx context.Context, endpoint string) error
}

type repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) Repository {
	return &repository{db: db}
}

func (r repository) Upsert(ctx context.Context, sub *entities.PushSubscription) error {
	const query = `
		INSERT INTO push_subscriptions (endpoint, p256dh, auth, last_seen)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (endpoint) DO UPDATE SET
			p256dh = EXCLUDED.p256dh,
			auth = EXCLUDED.auth,
			last_seen = now()`

	if _, err := r.db.Pool.Exec(ctx, query, sub.Endpoint, sub.P256dh, sub.Auth); err != nil {
		return fmt.Errorf("upsert push subscription: %w", err)
	}

	return nil
}

func (r repository) List(ctx context.Context) ([]entities.PushSubscription, error) {
	const query = `SELECT endpoint, p256dh, auth FROM push_subscriptions ORDER BY created_at`

	rows, err := r.db.Pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list push subscriptions: %w", err)
	}

	subs, err := pgx.CollectRows(rows, pgx.RowToStructByName[entities.PushSubscription])
	if err != nil {
		return nil, fmt.Errorf("list push subscriptions: %w", err)
	}

	return subs, nil
}

func (r repository) Delete(ctx context.Context, endpoint string) error {
	if _, err := r.db.Pool.Exec(ctx, `DELETE FROM push_subscriptions WHERE endpoint = $1`, endpoint); err != nil {
		return fmt.Errorf("delete push subscription: %w", err)
	}

	return nil
}
