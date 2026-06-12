package handlers

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/oapi-codegen/nullable"
	"go.uber.org/zap"

	"finance/generated/api"
	"finance/internal/entities"
	"finance/internal/ledger"
	accountrepository "finance/internal/repositories/account_repository"
	categoryrepository "finance/internal/repositories/category_repository"
	transactionrepository "finance/internal/repositories/transaction_repository"
	"finance/pkg/fx"
	"finance/pkg/money"
)

func (s Server) CreateTransaction(ctx context.Context, request api.CreateTransactionRequestObject) (api.CreateTransactionResponseObject, error) {
	if request.Body == nil {
		return api.CreateTransaction400JSONResponse{BadRequestJSONResponse: badRequest("empty body")}, nil
	}
	body := request.Body

	in := ledgerInput(body)

	// resolve referenced buckets; a missing reference is a client error
	if body.FromAccountId != nil {
		acc, resp := s.loadAccount(ctx, *body.FromAccountId)
		if resp != nil {
			return resp, nil
		}
		in.From = acc
	}
	if body.ToAccountId != nil {
		acc, resp := s.loadAccount(ctx, *body.ToAccountId)
		if resp != nil {
			return resp, nil
		}
		in.To = acc
	}
	if body.CategoryId != nil {
		c, resp := s.loadCategory(ctx, *body.CategoryId)
		if resp != nil {
			return resp, nil
		}
		in.Category = c
	}
	if body.RateToBase != nil {
		rate, err := fx.ParseRate(*body.RateToBase)
		if err != nil {
			return api.CreateTransaction400JSONResponse{BadRequestJSONResponse: badRequest("invalid rate_to_base")}, nil
		}
		in.RateToBase = &rate
	}

	tx, err := ledger.BuildTransaction(in, s.base)
	if err != nil {
		return api.CreateTransaction400JSONResponse{BadRequestJSONResponse: badRequest(err.Error())}, nil
	}

	if err = s.transactions.Create(ctx, &tx); err != nil {
		s.logger.Error("create transaction", zap.Error(err))

		return nil, err
	}

	return api.CreateTransaction201JSONResponse(s.toTransaction(tx)), nil
}

func (s Server) ListTransactions(ctx context.Context, request api.ListTransactionsRequestObject) (api.ListTransactionsResponseObject, error) {
	p := request.Params
	filter := transactionrepository.Filter{
		AccountID:  p.AccountId,
		CategoryID: p.CategoryId,
		DateFrom:   p.DateFrom,
		DateTo:     p.DateTo,
		Tag:        p.Tag,
		Query:      p.Q,
	}
	if p.Type != nil {
		t := entities.TransactionType(*p.Type)
		filter.Type = &t
	}
	if p.Limit != nil {
		filter.Limit = *p.Limit
	}
	if p.Offset != nil {
		filter.Offset = *p.Offset
	}

	txns, err := s.transactions.List(ctx, filter)
	if err != nil {
		return nil, err
	}

	out := make([]api.Transaction, len(txns))
	for i, t := range txns {
		out[i] = s.toTransaction(t)
	}

	return api.ListTransactions200JSONResponse{Transactions: out}, nil
}

func (s Server) GetTransaction(ctx context.Context, request api.GetTransactionRequestObject) (api.GetTransactionResponseObject, error) {
	tx, err := s.transactions.Get(ctx, request.Id)
	if errors.Is(err, transactionrepository.ErrNotFound) {
		return api.GetTransaction404JSONResponse{NotFoundJSONResponse: notFound("transaction not found")}, nil
	}
	if err != nil {
		return nil, err
	}

	return api.GetTransaction200JSONResponse(s.toTransaction(*tx)), nil
}

func (s Server) DeleteTransaction(ctx context.Context, request api.DeleteTransactionRequestObject) (api.DeleteTransactionResponseObject, error) {
	err := s.transactions.Delete(ctx, request.Id)
	if errors.Is(err, transactionrepository.ErrNotFound) {
		return api.DeleteTransaction404JSONResponse{NotFoundJSONResponse: notFound("transaction not found")}, nil
	}
	if err != nil {
		return nil, err
	}

	return api.DeleteTransaction204Response{}, nil
}

func ledgerInput(body *api.CreateTransactionJSONRequestBody) ledger.NewTransaction {
	date := time.Now()
	if body.Date != nil {
		date = *body.Date
	}

	var tags []string
	if body.Tags != nil {
		tags = *body.Tags
	}

	return ledger.NewTransaction{
		Date:     date,
		Type:     entities.TransactionType(body.Type),
		Amount:   body.Amount,
		ToAmount: body.ToAmount,
		Note:     body.Note,
		Tags:     tags,
	}
}

func (s Server) loadAccount(ctx context.Context, id uuid.UUID) (*entities.Account, api.CreateTransactionResponseObject) {
	acc, err := s.accounts.Get(ctx, id)
	if errors.Is(err, accountrepository.ErrNotFound) {
		return nil, api.CreateTransaction400JSONResponse{BadRequestJSONResponse: badRequest("account not found: " + id.String())}
	}
	if err != nil {
		s.logger.Error("load account", zap.Error(err))

		return nil, api.CreateTransaction400JSONResponse{BadRequestJSONResponse: badRequest("invalid account")}
	}

	return acc, nil
}

func (s Server) loadCategory(ctx context.Context, id uuid.UUID) (*entities.Category, api.CreateTransactionResponseObject) {
	c, err := s.categories.Get(ctx, id)
	if errors.Is(err, categoryrepository.ErrNotFound) {
		return nil, api.CreateTransaction400JSONResponse{BadRequestJSONResponse: badRequest("category not found: " + id.String())}
	}
	if err != nil {
		s.logger.Error("load category", zap.Error(err))

		return nil, api.CreateTransaction400JSONResponse{BadRequestJSONResponse: badRequest("invalid category")}
	}

	return c, nil
}

func (s Server) toTransaction(tx entities.Transaction) api.Transaction {
	tags := tx.Tags
	if tags == nil {
		tags = []string{}
	}

	out := api.Transaction{
		Id:        tx.ID,
		Date:      tx.Date,
		Type:      api.TransactionType(tx.Type),
		Amount:    money.New(tx.Amount, tx.Currency),
		Tags:      tags,
		CreatedAt: tx.CreatedAt,
	}

	if tx.FromAccountID != nil {
		out.FromAccountId = nullable.NewNullableWithValue(*tx.FromAccountID)
	}
	if tx.ToAccountID != nil {
		out.ToAccountId = nullable.NewNullableWithValue(*tx.ToAccountID)
	}
	if tx.CategoryID != nil {
		out.CategoryId = nullable.NewNullableWithValue(*tx.CategoryID)
	}
	if tx.ToAmount != nil && tx.ToCurrency != nil {
		m := money.New(*tx.ToAmount, *tx.ToCurrency)
		out.ToAmount = &m
	}
	if tx.RateToBase != nil {
		out.RateToBase = nullable.NewNullableWithValue(tx.RateToBase.String())
	}
	if tx.BaseAmount != nil {
		m := money.New(*tx.BaseAmount, s.base)
		out.BaseAmount = &m
	}
	if tx.Note != nil {
		out.Note = nullable.NewNullableWithValue(*tx.Note)
	}

	return out
}
