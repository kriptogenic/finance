package balancesnapshotrepository

import (
	"context"
	"fmt"

	"finance/internal/entities"
	"finance/pkg/database"
)

type Repository interface {
	// Upsert stores the latest reported balance for a card, keyed by card_last4.
	Upsert(ctx context.Context, snap *entities.BalanceSnapshot) error
	List(ctx context.Context) ([]entities.BalanceSnapshot, error)
}

type repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) Repository {
	return &repository{db: db}
}

func (r repository) Upsert(ctx context.Context, snap *entities.BalanceSnapshot) error {
	const query = `
		INSERT INTO balance_snapshots (card_last4, bank, amount, source, reported_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, now())
		ON CONFLICT (card_last4) DO UPDATE SET
			bank = COALESCE(EXCLUDED.bank, balance_snapshots.bank),
			amount = EXCLUDED.amount,
			source = EXCLUDED.source,
			reported_at = EXCLUDED.reported_at,
			updated_at = now()`

	_, err := r.db.Pool.Exec(ctx, query,
		snap.CardLast4, snap.Bank, snap.Amount, snap.Source, snap.ReportedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert balance snapshot: %w", err)
	}

	return nil
}

func (r repository) List(ctx context.Context) ([]entities.BalanceSnapshot, error) {
	const query = `SELECT card_last4, bank, amount, source, reported_at
		FROM balance_snapshots ORDER BY card_last4`

	rows, err := r.db.Pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list balance snapshots: %w", err)
	}
	defer rows.Close()

	var snaps []entities.BalanceSnapshot
	for rows.Next() {
		var s entities.BalanceSnapshot
		if err = rows.Scan(&s.CardLast4, &s.Bank, &s.Amount, &s.Source, &s.ReportedAt); err != nil {
			return nil, fmt.Errorf("list balance snapshots: %w", err)
		}

		snaps = append(snaps, s)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("list balance snapshots: %w", err)
	}

	return snaps, nil
}
