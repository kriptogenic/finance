package ingest

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"finance/config"
	"finance/internal/entities"
	"finance/internal/ledger"
	"finance/internal/pushnotify"
	accountrepository "finance/internal/repositories/account_repository"
	categoryrepository "finance/internal/repositories/category_repository"
	transactionrepository "finance/internal/repositories/transaction_repository"
	"finance/pkg/fx"
	"finance/pkg/money"
)

type ValidationError struct{ msg string }

func (e ValidationError) Error() string { return e.msg }

type Command struct {
	ExternalID      string
	Date            *time.Time
	Type            entities.TransactionType
	Amount          int64
	ToAmount        *int64
	RateToBase      *string
	Merchant        *string
	Tags            []string
	FromCardLast4   *string
	ToCardLast4     *string
	TransferGroupID *string
}

type Result struct {
	Transaction entities.Transaction
	Created     bool
}

type Service struct {
	accounts     accountrepository.Repository
	categories   categoryrepository.Repository
	transactions transactionrepository.Repository
	notifier     pushnotify.Notifier
	base         string
	pairWindow   time.Duration
	logger       *zap.Logger
}

func NewService(
	accounts accountrepository.Repository,
	categories categoryrepository.Repository,
	transactions transactionrepository.Repository,
	notifier pushnotify.Notifier,
	finance *config.Finance,
	logger *zap.Logger,
) *Service {
	return &Service{
		accounts:     accounts,
		categories:   categories,
		transactions: transactions,
		notifier:     notifier,
		base:         finance.BaseCurrency,
		pairWindow:   finance.TransferPairWindow,
		logger:       logger,
	}
}

func (s *Service) Ingest(ctx context.Context, cmd Command) (Result, error) {
	// Idempotency: a re-delivered leg that was already swallowed into a transfer,
	// or already committed on its own, returns the stored transaction unchanged.
	if tx, err := s.transactions.ConsumedTransfer(ctx, cmd.ExternalID); err == nil {
		return Result{Transaction: *tx, Created: false}, nil
	} else if !errors.Is(err, transactionrepository.ErrNotFound) {
		return Result{}, fmt.Errorf("consumed leg lookup: %w", err)
	}
	if tx, err := s.transactions.ByExternalID(ctx, cmd.ExternalID); err == nil {
		return Result{Transaction: *tx, Created: false}, nil
	} else if !errors.Is(err, transactionrepository.ErrNotFound) {
		return Result{}, fmt.Errorf("external_id lookup: %w", err)
	}

	date := time.Now()
	if cmd.Date != nil {
		date = *cmd.Date
	}

	in := ledger.NewTransaction{
		Date: date,
		Type: cmd.Type,
		Note: cmd.Merchant,
		Tags: cmd.Tags,
	}

	if cmd.FromCardLast4 != nil {
		acc, err := s.resolveCard(ctx, *cmd.FromCardLast4)
		if err != nil {
			return Result{}, err
		}
		in.From = acc
	}
	if cmd.ToCardLast4 != nil {
		acc, err := s.resolveCard(ctx, *cmd.ToCardLast4)
		if err != nil {
			return Result{}, err
		}
		in.To = acc
	}

	// the amount's currency comes from the resolved account (the engine re-derives
	// it too); income credits the to account, everything else debits from.
	cur := s.base
	switch {
	case in.From != nil:
		cur = in.From.Currency
	case in.To != nil:
		cur = in.To.Currency
	}
	in.Amount = money.New(cmd.Amount, cur)
	if cmd.ToAmount != nil {
		toCur := s.base
		if in.To != nil {
			toCur = in.To.Currency
		}
		m := money.New(*cmd.ToAmount, toCur)
		in.ToAmount = &m
	}

	var catID *uuid.UUID
	if cmd.Type != entities.TxTransfer {
		cat, err := s.resolveCategory(ctx, cmd.Type, cmd.Merchant)
		if err != nil {
			return Result{}, err
		}
		in.Category = cat
		catID = &cat.ID
	}

	if cmd.RateToBase != nil {
		rate, err := fx.ParseRate(*cmd.RateToBase)
		if err != nil {
			return Result{}, ValidationError{"invalid rate_to_base"}
		}
		in.RateToBase = &rate
	}

	tx, err := ledger.BuildTransaction(in, s.base)
	if err != nil {
		return Result{}, ValidationError{err.Error()}
	}

	tx.ExternalID = &cmd.ExternalID
	tx.TransferGroupID = cmd.TransferGroupID

	// A bare expense/income leg may be one half of a card-to-card transfer whose
	// other half already arrived (possibly from another source). If so, merge the
	// pair into a single transfer instead of committing this leg on its own.
	if transfer, ok, err := s.tryPair(ctx, tx); err != nil {
		return Result{}, err
	} else if ok {
		return Result{Transaction: *transfer, Created: true}, nil
	}

	created, err := s.transactions.Ingest(ctx, &tx)
	if err != nil {
		return Result{}, fmt.Errorf("ingest transaction: %w", err)
	}

	if created && catID != nil {
		s.notify(tx, *catID)
	}

	return Result{Transaction: tx, Created: created}, nil
}

// tryPair looks for an already-committed opposite leg that, together with the
// just-built leg, forms a transfer. On a match it rewrites the existing leg's
// row into the transfer and records this leg's external_id as consumed, so this
// leg never becomes its own row. It returns ok=false for non-expense/income
// legs or when no mate exists.
func (s *Service) tryPair(ctx context.Context, leg entities.Transaction) (*entities.Transaction, bool, error) {
	var legAccount *uuid.UUID
	var opposite entities.TransactionType
	switch leg.Type {
	case entities.TxExpense:
		legAccount, opposite = leg.FromAccountID, entities.TxIncome
	case entities.TxIncome:
		legAccount, opposite = leg.ToAccountID, entities.TxExpense
	default:
		return nil, false, nil
	}
	if legAccount == nil {
		return nil, false, nil
	}

	mate, err := s.transactions.FindTransferMate(ctx, transactionrepository.MateQuery{
		OppositeType:     opposite,
		AmountMinor:      leg.Amount.Minor(),
		Currency:         leg.Amount.Code(),
		ExcludeAccountID: *legAccount,
		From:             leg.Date.Add(-s.pairWindow),
		To:               leg.Date.Add(s.pairWindow),
		Around:           leg.Date,
	})
	if errors.Is(err, transactionrepository.ErrNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("find transfer mate: %w", err)
	}

	transfer, err := ledger.TransferFromLegs(*mate, leg)
	if err != nil {
		// The mate passed the SQL guards but the ledger rejected the pair; treat
		// it as no match and commit this leg on its own.
		s.logger.Warn("transfer mate rejected by ledger", zap.Error(err))

		return nil, false, nil
	}

	if err := s.transactions.MergeIntoTransfer(ctx, mate.ID, &transfer, *leg.ExternalID); err != nil {
		return nil, false, fmt.Errorf("merge into transfer: %w", err)
	}

	s.logger.Info("paired ingest legs into a transfer",
		zap.String("transfer_id", transfer.ID.String()),
		zap.String("kept_leg", *mate.ExternalID),
		zap.String("consumed_leg", *leg.ExternalID),
	)

	return &transfer, true, nil
}

func (s *Service) resolveCard(ctx context.Context, last4 string) (*entities.Account, error) {
	acc, err := s.accounts.ByCardLast4(ctx, last4)
	if errors.Is(err, accountrepository.ErrNotFound) {
		return nil, ValidationError{"unknown card: " + last4}
	}
	if err != nil {
		return nil, fmt.Errorf("resolve card: %w", err)
	}

	return acc, nil
}

func (s *Service) resolveCategory(ctx context.Context, t entities.TransactionType, merchantPtr *string) (*entities.Category, error) {
	catType := entities.CategoryExpense
	if t == entities.TxIncome {
		catType = entities.CategoryIncome
	}
	merchant := ""
	if merchantPtr != nil {
		merchant = *merchantPtr
	}

	catID, err := s.categories.ResolveForIngest(ctx, catType, merchant)
	if err != nil {
		s.logger.Error("resolve ingest category", zap.Error(err))

		return nil, ValidationError{"could not resolve a category (seed the Uncategorized buckets)"}
	}

	cat, err := s.categories.Get(ctx, catID)
	if err != nil {
		return nil, fmt.Errorf("load resolved category: %w", err)
	}

	return cat, nil
}

func (s *Service) notify(tx entities.Transaction, catID uuid.UUID) {
	merchant := ""
	if tx.Note != nil {
		merchant = *tx.Note
	}
	s.notifier.OnIngestedCategory(pushnotify.Ingested{
		CategoryID: catID,
		Merchant:   merchant,
		Amount:     tx.Amount.Display(),
	})
}
