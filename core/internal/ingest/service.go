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
		logger:       logger,
	}
}

func (s *Service) Ingest(ctx context.Context, cmd Command) (Result, error) {
	date := time.Now()
	if cmd.Date != nil {
		date = *cmd.Date
	}

	in := ledger.NewTransaction{
		Date:     date,
		Type:     cmd.Type,
		Amount:   cmd.Amount,
		ToAmount: cmd.ToAmount,
		Note:     cmd.Merchant,
		Tags:     cmd.Tags,
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

	created, err := s.transactions.Ingest(ctx, &tx)
	if err != nil {
		return Result{}, fmt.Errorf("ingest transaction: %w", err)
	}

	if created && catID != nil {
		s.notify(tx, *catID)
	}

	return Result{Transaction: tx, Created: created}, nil
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
		Amount:     money.New(tx.Amount, tx.Currency).Display(),
	})
}
