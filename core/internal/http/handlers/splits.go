package handlers

import (
	"context"
	"errors"

	"github.com/oapi-codegen/nullable"
	"go.uber.org/zap"

	"finance/generated/api"
	"finance/internal/entities"
	"finance/internal/ledger"
	splitrepository "finance/internal/repositories/split_repository"
	transactionrepository "finance/internal/repositories/transaction_repository"
	"finance/pkg/money"
)

func (s Server) GetTransactionSplit(ctx context.Context, request api.GetTransactionSplitRequestObject) (api.GetTransactionSplitResponseObject, error) {
	tx, err := s.transactions.Get(ctx, request.Id)
	if errors.Is(err, transactionrepository.ErrNotFound) {
		return api.GetTransactionSplit404JSONResponse{NotFoundJSONResponse: notFound("transaction not found")}, nil
	}
	if err != nil {
		return nil, err
	}

	main, err := s.resolveSplitMain(ctx, tx)
	if err != nil {
		return nil, err
	}

	out, err := s.splitResponse(ctx, main)
	if err != nil {
		return nil, err
	}

	return api.GetTransactionSplit200JSONResponse(out), nil
}

func (s Server) SetTransactionSplit(ctx context.Context, request api.SetTransactionSplitRequestObject) (api.SetTransactionSplitResponseObject, error) {
	if request.Body == nil {
		return api.SetTransactionSplit400JSONResponse{BadRequestJSONResponse: badRequest("empty body")}, nil
	}

	tx, err := s.transactions.Get(ctx, request.Id)
	if errors.Is(err, transactionrepository.ErrNotFound) {
		return api.SetTransactionSplit404JSONResponse{NotFoundJSONResponse: notFound("transaction not found")}, nil
	}
	if err != nil {
		return nil, err
	}

	if tx.Type != entities.TxExpense || tx.FromAccountID == nil {
		return api.SetTransactionSplit400JSONResponse{BadRequestJSONResponse: badRequest("only an expense can be split")}, nil
	}

	paying, msg := s.getAccount(ctx, *tx.FromAccountID)
	if msg != "" {
		return api.SetTransactionSplit400JSONResponse{BadRequestJSONResponse: badRequest(msg)}, nil
	}

	participants := make([]ledger.SplitParticipant, len(request.Body.Participants))
	for i, p := range request.Body.Participants {
		participants[i] = ledger.SplitParticipant{Name: p.Name, Amount: p.Amount}
	}

	if err = ledger.ValidateSplit(request.Body.MyShare, participants); err != nil {
		return api.SetTransactionSplit400JSONResponse{BadRequestJSONResponse: badRequest(err.Error())}, nil
	}

	// re-freeze base_amount for the reduced expense (no-op when in base currency)
	var myShareBase *int64
	if tx.RateToBase != nil {
		b := ledger.FreezeBase(request.Body.MyShare, *tx.RateToBase)
		myShareBase = &b
	}

	if _, err = s.splits.Apply(ctx, splitrepository.ApplyParams{
		MainTxID:      tx.ID,
		PayingAccount: *paying,
		MyShare:       request.Body.MyShare,
		MyShareBase:   myShareBase,
		Participants:  participants,
	}); err != nil {
		if errors.Is(err, splitrepository.ErrNotFound) {
			return api.SetTransactionSplit404JSONResponse{NotFoundJSONResponse: notFound("transaction not found")}, nil
		}
		s.logger.Error("apply split", zap.Error(err))

		return nil, err
	}

	main, err := s.transactions.Get(ctx, tx.ID)
	if err != nil {
		return nil, err
	}

	out, err := s.splitResponse(ctx, main)
	if err != nil {
		return nil, err
	}

	return api.SetTransactionSplit200JSONResponse(out), nil
}

// resolveSplitMain returns the expense leg of tx's split. tx may be the expense
// itself or one of its per-person transfer legs.
func (s Server) resolveSplitMain(ctx context.Context, tx *entities.Transaction) (*entities.Transaction, error) {
	if tx.SplitGroupID == nil || tx.Type == entities.TxExpense {
		return tx, nil
	}

	legs, err := s.transactions.ListBySplitGroup(ctx, *tx.SplitGroupID)
	if err != nil {
		return nil, err
	}
	for i := range legs {
		if legs[i].Type == entities.TxExpense {
			return &legs[i], nil
		}
	}

	return tx, nil
}

// splitResponse builds the breakdown for a (resolved) main expense: your share
// plus each person's share and what they still owe you.
func (s Server) splitResponse(ctx context.Context, main *entities.Transaction) (api.TransactionSplit, error) {
	out := api.TransactionSplit{
		MyShare:      money.New(main.Amount, main.Currency),
		Participants: []api.SplitParticipant{},
	}
	if main.SplitGroupID == nil {
		return out, nil
	}
	out.SplitGroupId = nullable.NewNullableWithValue(main.SplitGroupID.String())

	legs, err := s.transactions.ListBySplitGroup(ctx, *main.SplitGroupID)
	if err != nil {
		return api.TransactionSplit{}, err
	}
	balances, err := s.accounts.Balances(ctx)
	if err != nil {
		return api.TransactionSplit{}, err
	}

	for _, leg := range legs {
		if leg.ID == main.ID || leg.ToAccountID == nil {
			continue
		}
		acc, gMsg := s.getAccount(ctx, *leg.ToAccountID)
		if gMsg != "" {
			continue
		}
		out.Participants = append(out.Participants, api.SplitParticipant{
			AccountId: acc.ID,
			Name:      acc.Name,
			Amount:    money.New(leg.Amount, leg.Currency),
			Owed:      money.New(balances[acc.ID], acc.Currency),
		})
	}

	return out, nil
}
