// Package splitrepository owns the atomic write that turns an expense into a
// split: it shrinks the main expense to your share and creates one receivable
// account + transfer leg per friend, all in a single transaction. It spans the
// accounts and transactions aggregates, so it lives in its own repository.
package splitrepository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"finance/internal/entities"
	"finance/internal/ledger"
	"finance/pkg/database"
	"finance/pkg/money"
)

var ErrNotFound = errors.New("transaction not found")

// ApplyParams is the resolved input to Apply.
type ApplyParams struct {
	MainTxID      uuid.UUID
	PayingAccount entities.Account // friend legs and receivable accounts use its currency
	MyShare       money.Money
	MyShareBase   *money.Money // frozen base amount for MyShare; nil when the expense is in base currency
	Participants  []ledger.SplitParticipant
}

type Repository interface {
	// Apply replaces the split of the main expense: it removes any prior friend
	// legs (and their now-orphan receivable accounts), sets the expense amount to
	// MyShare, and creates a receivable account + transfer leg per participant.
	// An empty participant list un-splits the expense. Returns the new split
	// group id (nil when un-split).
	Apply(ctx context.Context, p ApplyParams) (*uuid.UUID, error)
}

type repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) Repository {
	return &repository{db: db}
}

func (r repository) Apply(ctx context.Context, p ApplyParams) (group *uuid.UUID, err error) {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("split begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var (
		date     interface{}
		oldGroup *uuid.UUID
	)
	err = tx.QueryRow(ctx,
		`SELECT date, split_group_id FROM transactions WHERE id = $1 FOR UPDATE`,
		p.MainTxID).Scan(&date, &oldGroup)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("split load main: %w", err)
	}

	if err = clearGroup(ctx, tx, p.MainTxID, oldGroup); err != nil {
		return nil, err
	}

	if len(p.Participants) == 0 {
		if _, err = tx.Exec(ctx,
			`UPDATE transactions SET amount = $2, base_amount = $3, split_group_id = NULL WHERE id = $1`,
			p.MainTxID, p.MyShare.Minor(), splitMinor(p.MyShareBase)); err != nil {
			return nil, fmt.Errorf("split unsplit: %w", err)
		}

		return nil, commit(ctx, tx)
	}

	newGroup := uuid.New()
	if _, err = tx.Exec(ctx,
		`UPDATE transactions SET amount = $2, base_amount = $3, split_group_id = $4 WHERE id = $1`,
		p.MainTxID, p.MyShare.Minor(), splitMinor(p.MyShareBase), newGroup); err != nil {
		return nil, fmt.Errorf("split main update: %w", err)
	}

	cur := p.PayingAccount.Currency
	for _, part := range p.Participants {
		var accID uuid.UUID
		if err = tx.QueryRow(ctx,
			`INSERT INTO accounts (name, kind, type, currency)
			 VALUES ($1, 'asset', 'receivable', $2) RETURNING id`,
			part.Name, cur).Scan(&accID); err != nil {
			return nil, fmt.Errorf("split create person: %w", err)
		}

		if _, err = tx.Exec(ctx,
			`INSERT INTO transactions
			   (date, type, from_account_id, to_account_id, amount, currency, split_group_id)
			 VALUES ($1, 'transfer', $2, $3, $4, $5, $6)`,
			date, p.PayingAccount.ID, accID, part.Amount.Minor(), cur, newGroup); err != nil {
			return nil, fmt.Errorf("split create leg: %w", err)
		}
	}

	return &newGroup, commit(ctx, tx)
}

// clearGroup removes the prior friend legs of group and deletes the receivable
// accounts they created, skipping any still referenced by other transactions
// (e.g. a partial repayment) so we never orphan a FK.
func clearGroup(ctx context.Context, tx pgx.Tx, mainID uuid.UUID, group *uuid.UUID) error {
	if group == nil {
		return nil
	}

	rows, err := tx.Query(ctx,
		`SELECT to_account_id FROM transactions
		 WHERE split_group_id = $1 AND id <> $2 AND to_account_id IS NOT NULL`,
		*group, mainID)
	if err != nil {
		return fmt.Errorf("split list legs: %w", err)
	}
	var personIDs []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			rows.Close()

			return fmt.Errorf("split scan leg: %w", err)
		}
		personIDs = append(personIDs, id)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return fmt.Errorf("split list legs: %w", err)
	}

	if _, err = tx.Exec(ctx,
		`DELETE FROM transactions WHERE split_group_id = $1 AND id <> $2`,
		*group, mainID); err != nil {
		return fmt.Errorf("split delete legs: %w", err)
	}

	// drop the per-person accounts now that their only leg is gone, unless some
	// other transaction (a repayment) still references them
	if len(personIDs) > 0 {
		if _, err = tx.Exec(ctx,
			`DELETE FROM accounts a
			 WHERE a.id = ANY($1) AND a.type = 'receivable'
			   AND NOT EXISTS (
			     SELECT 1 FROM transactions t
			     WHERE t.from_account_id = a.id OR t.to_account_id = a.id)`,
			personIDs); err != nil {
			return fmt.Errorf("split delete persons: %w", err)
		}
	}

	return nil
}

func splitMinor(m *money.Money) *int64 {
	if m == nil {
		return nil
	}
	v := m.Minor()

	return &v
}

func commit(ctx context.Context, tx pgx.Tx) error {
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("split commit: %w", err)
	}

	return nil
}
