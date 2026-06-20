// Package scheduler materializes scheduled (recurring) transactions into real
// transactions and runs the background worker that does so on a cadence.
package scheduler

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"finance/config"
	"finance/internal/entities"
	"finance/internal/ledger"
	accountrepository "finance/internal/repositories/account_repository"
	categoryrepository "finance/internal/repositories/category_repository"
	scheduledtransactionrepository "finance/internal/repositories/scheduled_transaction_repository"
	transactionrepository "finance/internal/repositories/transaction_repository"
)

// Materializer turns a ScheduledTransaction into a real Transaction, reusing the
// ledger engine so a materialized row honors the same money invariants as a
// manual one (§3/§4/§5).
type Materializer struct {
	accounts     accountrepository.Repository
	categories   categoryrepository.Repository
	transactions transactionrepository.Repository
	schedules    scheduledtransactionrepository.Repository
	base         string
	logger       *zap.Logger
}

func NewMaterializer(
	accounts accountrepository.Repository,
	categories categoryrepository.Repository,
	transactions transactionrepository.Repository,
	schedules scheduledtransactionrepository.Repository,
	finance *config.Finance,
	logger *zap.Logger,
) *Materializer {
	return &Materializer{
		accounts:     accounts,
		categories:   categories,
		transactions: transactions,
		schedules:    schedules,
		base:         finance.BaseCurrency,
		logger:       logger,
	}
}

// Run materializes a single occurrence of s (dated its NextRun), persists the
// transaction, and advances the schedule. When skipAhead is set, next_run is
// then rolled forward past asOf without creating further transactions — the
// "one then skip ahead" policy used to collapse missed occurrences.
func (m *Materializer) Run(ctx context.Context, s entities.ScheduledTransaction, asOf time.Time, skipAhead bool) (entities.Transaction, error) {
	tx, err := m.build(ctx, s)
	if err != nil {
		return entities.Transaction{}, err
	}

	if err = m.transactions.Create(ctx, &tx); err != nil {
		return entities.Transaction{}, fmt.Errorf("create transaction: %w", err)
	}

	next := entities.NextRun(s.Frequency, s.Interval, s.NextRun)
	if skipAhead {
		for !next.After(asOf) {
			next = entities.NextRun(s.Frequency, s.Interval, next)
		}
	}

	if err = m.schedules.Advance(ctx, s.ID, next, asOf); err != nil {
		return entities.Transaction{}, fmt.Errorf("advance schedule: %w", err)
	}

	return tx, nil
}

// RunDue materializes every schedule due by asOf and returns how many fired. A
// failing schedule is logged and skipped so one bad row can't wedge the loop.
func (m *Materializer) RunDue(ctx context.Context, asOf time.Time) (int, error) {
	due, err := m.schedules.Due(ctx, asOf)
	if err != nil {
		return 0, fmt.Errorf("load due schedules: %w", err)
	}

	fired := 0
	for _, s := range due {
		if _, err := m.Run(ctx, s, asOf, true); err != nil {
			m.logger.Error("materialize schedule", zap.String("id", s.ID.String()), zap.Error(err))

			continue
		}
		fired++
	}

	return fired, nil
}

// Validate checks that a schedule's template resolves its buckets and builds
// cleanly through the ledger engine. Returns a client-grade error (map to 400).
func (m *Materializer) Validate(ctx context.Context, s entities.ScheduledTransaction) error {
	_, err := m.build(ctx, s)

	return err
}

// build resolves the schedule's buckets and runs the ledger engine, dating the
// transaction at the schedule's NextRun. Every returned error is client-grade
// (bad template) — callers map it to 400 / log-and-skip.
func (m *Materializer) build(ctx context.Context, s entities.ScheduledTransaction) (entities.Transaction, error) {
	in := ledger.NewTransaction{
		Date:       s.NextRun,
		Type:       s.Type,
		Amount:     s.Amount,
		ToAmount:   s.ToAmount,
		RateToBase: s.RateToBase,
		Note:       s.Note,
		Tags:       s.Tags,
	}

	if s.FromAccountID != nil {
		acc, err := m.accounts.Get(ctx, *s.FromAccountID)
		if err != nil {
			return entities.Transaction{}, fmt.Errorf("from account: %w", err)
		}
		in.From = acc
	}
	if s.ToAccountID != nil {
		acc, err := m.accounts.Get(ctx, *s.ToAccountID)
		if err != nil {
			return entities.Transaction{}, fmt.Errorf("to account: %w", err)
		}
		in.To = acc
	}
	if s.CategoryID != nil {
		c, err := m.categories.Get(ctx, *s.CategoryID)
		if err != nil {
			return entities.Transaction{}, fmt.Errorf("category: %w", err)
		}
		in.Category = c
	}

	return ledger.BuildTransaction(in, m.base)
}
