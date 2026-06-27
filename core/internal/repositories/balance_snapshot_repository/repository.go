package balancesnapshotrepository

import (
	"context"
	"fmt"

	"finance/internal/entities"
	"finance/pkg/database"
	"finance/pkg/money"
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
		INSERT INTO balance_snapshots (card_last4, bank, amount, currency, source, reported_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, now())
		ON CONFLICT (card_last4) DO UPDATE SET
			bank = COALESCE(EXCLUDED.bank, balance_snapshots.bank),
			amount = EXCLUDED.amount,
			currency = EXCLUDED.currency,
			source = EXCLUDED.source,
			reported_at = EXCLUDED.reported_at,
			updated_at = now()`

	_, err := r.db.Pool.Exec(ctx, query,
		snap.CardLast4, snap.Bank, snap.Amount.Minor(), snap.Amount.Code(), snap.Source, snap.ReportedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert balance snapshot: %w", err)
	}

	return nil
}

func (r repository) List(ctx context.Context) ([]entities.BalanceSnapshot, error) {
	const query = `SELECT card_last4, bank, amount, currency, source, reported_at
		FROM balance_snapshots ORDER BY card_last4`

	rows, err := r.db.Pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list balance snapshots: %w", err)
	}
	defer rows.Close()

	var snaps []entities.BalanceSnapshot
	for rows.Next() {
		var (
			s        entities.BalanceSnapshot
			amount   int64
			currency string
		)
		if err = rows.Scan(&s.CardLast4, &s.Bank, &amount, &currency, &s.Source, &s.ReportedAt); err != nil {
			return nil, fmt.Errorf("list balance snapshots: %w", err)
		}
		s.Amount = money.New(amount, currency)

		snaps = append(snaps, s)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("list balance snapshots: %w", err)
	}

	return snaps, nil
}
