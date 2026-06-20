package handlers

import (
	"context"
	"errors"
	"time"

	"github.com/oapi-codegen/nullable"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"go.uber.org/zap"

	"finance/generated/api"
	"finance/internal/entities"
	scheduledtransactionrepository "finance/internal/repositories/scheduled_transaction_repository"
	"finance/pkg/fx"
	"finance/pkg/money"
)

func (s Server) ListScheduledTransactions(ctx context.Context, _ api.ListScheduledTransactionsRequestObject) (api.ListScheduledTransactionsResponseObject, error) {
	schedules, err := s.schedules.List(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]api.ScheduledTransaction, len(schedules))
	for i, sc := range schedules {
		resp, respErr := s.scheduleResponse(ctx, sc)
		if respErr != nil {
			return nil, respErr
		}
		out[i] = resp
	}

	return api.ListScheduledTransactions200JSONResponse{ScheduledTransactions: out}, nil
}

func (s Server) CreateScheduledTransaction(ctx context.Context, request api.CreateScheduledTransactionRequestObject) (api.CreateScheduledTransactionResponseObject, error) {
	if request.Body == nil {
		return api.CreateScheduledTransaction400JSONResponse{BadRequestJSONResponse: badRequest("empty body")}, nil
	}

	sc, msg := s.buildSchedule(ctx, request.Body)
	if msg != "" {
		return api.CreateScheduledTransaction400JSONResponse{BadRequestJSONResponse: badRequest(msg)}, nil
	}

	if err := s.schedules.Create(ctx, &sc); err != nil {
		s.logger.Error("create scheduled transaction", zap.Error(err))

		return nil, err
	}

	resp, err := s.scheduleResponse(ctx, sc)
	if err != nil {
		return nil, err
	}

	return api.CreateScheduledTransaction201JSONResponse(resp), nil
}

func (s Server) GetScheduledTransaction(ctx context.Context, request api.GetScheduledTransactionRequestObject) (api.GetScheduledTransactionResponseObject, error) {
	sc, err := s.schedules.Get(ctx, request.Id)
	if errors.Is(err, scheduledtransactionrepository.ErrNotFound) {
		return api.GetScheduledTransaction404JSONResponse{NotFoundJSONResponse: notFound("scheduled transaction not found")}, nil
	}
	if err != nil {
		return nil, err
	}

	resp, err := s.scheduleResponse(ctx, *sc)
	if err != nil {
		return nil, err
	}

	return api.GetScheduledTransaction200JSONResponse(resp), nil
}

func (s Server) UpdateScheduledTransaction(ctx context.Context, request api.UpdateScheduledTransactionRequestObject) (api.UpdateScheduledTransactionResponseObject, error) {
	existing, err := s.schedules.Get(ctx, request.Id)
	if errors.Is(err, scheduledtransactionrepository.ErrNotFound) {
		return api.UpdateScheduledTransaction404JSONResponse{NotFoundJSONResponse: notFound("scheduled transaction not found")}, nil
	}
	if err != nil {
		return nil, err
	}

	if request.Body == nil {
		return api.UpdateScheduledTransaction400JSONResponse{BadRequestJSONResponse: badRequest("empty body")}, nil
	}

	sc, msg := s.buildSchedule(ctx, updateToCreate(request.Body))
	if msg != "" {
		return api.UpdateScheduledTransaction400JSONResponse{BadRequestJSONResponse: badRequest(msg)}, nil
	}
	sc.ID = existing.ID
	sc.CreatedAt = existing.CreatedAt

	if err = s.schedules.Update(ctx, &sc); err != nil {
		if errors.Is(err, scheduledtransactionrepository.ErrNotFound) {
			return api.UpdateScheduledTransaction404JSONResponse{NotFoundJSONResponse: notFound("scheduled transaction not found")}, nil
		}
		s.logger.Error("update scheduled transaction", zap.Error(err))

		return nil, err
	}

	resp, err := s.scheduleResponse(ctx, sc)
	if err != nil {
		return nil, err
	}

	return api.UpdateScheduledTransaction200JSONResponse(resp), nil
}

func (s Server) DeleteScheduledTransaction(ctx context.Context, request api.DeleteScheduledTransactionRequestObject) (api.DeleteScheduledTransactionResponseObject, error) {
	err := s.schedules.Delete(ctx, request.Id)
	if errors.Is(err, scheduledtransactionrepository.ErrNotFound) {
		return api.DeleteScheduledTransaction404JSONResponse{NotFoundJSONResponse: notFound("scheduled transaction not found")}, nil
	}
	if err != nil {
		return nil, err
	}

	return api.DeleteScheduledTransaction204Response{}, nil
}

func (s Server) RunScheduledTransaction(ctx context.Context, request api.RunScheduledTransactionRequestObject) (api.RunScheduledTransactionResponseObject, error) {
	sc, err := s.schedules.Get(ctx, request.Id)
	if errors.Is(err, scheduledtransactionrepository.ErrNotFound) {
		return api.RunScheduledTransaction404JSONResponse{NotFoundJSONResponse: notFound("scheduled transaction not found")}, nil
	}
	if err != nil {
		return nil, err
	}

	// manual run: post exactly one occurrence and advance a single step.
	tx, err := s.materializer.Run(ctx, *sc, time.Now().UTC(), false)
	if err != nil {
		s.logger.Error("run scheduled transaction", zap.Error(err))

		return api.RunScheduledTransaction400JSONResponse{BadRequestJSONResponse: badRequest(err.Error())}, nil
	}

	return api.RunScheduledTransaction201JSONResponse(s.toTransaction(tx)), nil
}

// buildSchedule validates the request and assembles the entity. It returns a
// non-empty message describing the first client error, or "" on success.
func (s Server) buildSchedule(ctx context.Context, body *api.CreateScheduledTransactionRequest) (entities.ScheduledTransaction, string) {
	if !body.Frequency.Valid() {
		return entities.ScheduledTransaction{}, "invalid frequency"
	}

	interval := 1
	if body.Interval != nil {
		interval = *body.Interval
	}
	if interval < 1 {
		return entities.ScheduledTransaction{}, "interval must be at least 1"
	}

	var tags []string
	if body.Tags != nil {
		tags = *body.Tags
	}

	sc := entities.ScheduledTransaction{
		Name:          body.Name,
		Type:          entities.TransactionType(body.Type),
		FromAccountID: body.FromAccountId,
		ToAccountID:   body.ToAccountId,
		CategoryID:    body.CategoryId,
		Amount:        body.Amount,
		ToAmount:      body.ToAmount,
		Note:          body.Note,
		Tags:          tags,
		Frequency:     entities.ScheduleFrequency(body.Frequency),
		Interval:      interval,
		NextRun:       body.NextRun.UTC(),
		Paused:        body.Paused != nil && *body.Paused,
	}
	if body.EndDate != nil {
		end := body.EndDate.UTC()
		sc.EndDate = &end
	}
	if body.RateToBase != nil {
		rate, err := fx.ParseRate(*body.RateToBase)
		if err != nil {
			return entities.ScheduledTransaction{}, "invalid rate_to_base"
		}
		sc.RateToBase = &rate
	}

	// dry-run the template through the ledger so bad shapes/currencies/rates are
	// rejected at create time, not silently at the first worker tick.
	if err := s.materializer.Validate(ctx, sc); err != nil {
		return entities.ScheduledTransaction{}, err.Error()
	}

	return sc, ""
}

func (s Server) scheduleResponse(ctx context.Context, sc entities.ScheduledTransaction) (api.ScheduledTransaction, error) {
	currency, toCurrency, err := s.scheduleCurrencies(ctx, sc)
	if err != nil {
		return api.ScheduledTransaction{}, err
	}

	tags := sc.Tags
	if tags == nil {
		tags = []string{}
	}

	out := api.ScheduledTransaction{
		Id:        sc.ID,
		Type:      api.TransactionType(sc.Type),
		Amount:    money.New(sc.Amount, currency),
		Tags:      tags,
		Frequency: api.ScheduleFrequency(sc.Frequency),
		Interval:  sc.Interval,
		NextRun:   openapi_types.Date{Time: sc.NextRun},
		Paused:    sc.Paused,
		CreatedAt: sc.CreatedAt,
	}

	if sc.Name != nil {
		out.Name = nullable.NewNullableWithValue(*sc.Name)
	}
	if sc.FromAccountID != nil {
		out.FromAccountId = nullable.NewNullableWithValue(*sc.FromAccountID)
	}
	if sc.ToAccountID != nil {
		out.ToAccountId = nullable.NewNullableWithValue(*sc.ToAccountID)
	}
	if sc.CategoryID != nil {
		out.CategoryId = nullable.NewNullableWithValue(*sc.CategoryID)
	}
	if sc.ToAmount != nil {
		m := money.New(*sc.ToAmount, toCurrency)
		out.ToAmount = &m
	}
	if sc.RateToBase != nil {
		out.RateToBase = nullable.NewNullableWithValue(sc.RateToBase.String())
	}
	if sc.Note != nil {
		out.Note = nullable.NewNullableWithValue(*sc.Note)
	}
	if sc.EndDate != nil {
		out.EndDate = nullable.NewNullableWithValue(openapi_types.Date{Time: *sc.EndDate})
	}
	if sc.LastRunAt != nil {
		out.LastRunAt = nullable.NewNullableWithValue(*sc.LastRunAt)
	}

	return out, nil
}

// scheduleCurrencies resolves the response money currencies: the primary leg's
// account currency (from for expense/transfer, to for income) and, for
// transfers, the credited account's currency.
func (s Server) scheduleCurrencies(ctx context.Context, sc entities.ScheduledTransaction) (primary, to string, err error) {
	primaryID := sc.FromAccountID
	if sc.Type == entities.TxIncome {
		primaryID = sc.ToAccountID
	}
	if primaryID != nil {
		acc, accErr := s.accounts.Get(ctx, *primaryID)
		if accErr != nil {
			return "", "", accErr
		}
		primary = acc.Currency
	}

	if sc.Type == entities.TxTransfer && sc.ToAccountID != nil {
		acc, accErr := s.accounts.Get(ctx, *sc.ToAccountID)
		if accErr != nil {
			return "", "", accErr
		}
		to = acc.Currency
	}

	return primary, to, nil
}

// updateToCreate adapts the (identical-shaped) update body to the create body so
// both paths share buildSchedule.
func updateToCreate(b *api.UpdateScheduledTransactionRequest) *api.CreateScheduledTransactionRequest {
	return &api.CreateScheduledTransactionRequest{
		Name:          b.Name,
		Type:          b.Type,
		FromAccountId: b.FromAccountId,
		ToAccountId:   b.ToAccountId,
		CategoryId:    b.CategoryId,
		Amount:        b.Amount,
		ToAmount:      b.ToAmount,
		RateToBase:    b.RateToBase,
		Note:          b.Note,
		Tags:          b.Tags,
		Frequency:     b.Frequency,
		Interval:      b.Interval,
		NextRun:       b.NextRun,
		EndDate:       b.EndDate,
		Paused:        b.Paused,
	}
}
