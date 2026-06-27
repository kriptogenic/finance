package handlers

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/oapi-codegen/nullable"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"go.uber.org/zap"

	"finance/generated/api"
	"finance/internal/entities"
	budgetrepository "finance/internal/repositories/budget_repository"
	categoryrepository "finance/internal/repositories/category_repository"
	"finance/pkg/money"
)

func (s Server) ListBudgets(ctx context.Context, _ api.ListBudgetsRequestObject) (api.ListBudgetsResponseObject, error) {
	budgets, err := s.budgets.List(ctx)
	if err != nil {
		return nil, err
	}

	names, err := s.categoryNames(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]api.Budget, len(budgets))
	for i, b := range budgets {
		resp, respErr := s.budgetResponse(ctx, b, names[b.CategoryID])
		if respErr != nil {
			return nil, respErr
		}
		out[i] = resp
	}

	return api.ListBudgets200JSONResponse{Budgets: out}, nil
}

func (s Server) CreateBudget(ctx context.Context, request api.CreateBudgetRequestObject) (api.CreateBudgetResponseObject, error) {
	if request.Body == nil {
		return api.CreateBudget400JSONResponse{BadRequestJSONResponse: badRequest("empty body")}, nil
	}
	body := request.Body

	cat, err := s.categories.Get(ctx, body.CategoryId)
	if errors.Is(err, categoryrepository.ErrNotFound) {
		return api.CreateBudget400JSONResponse{BadRequestJSONResponse: badRequest("category not found")}, nil
	}
	if err != nil {
		return nil, err
	}
	if cat.Type != entities.CategoryExpense {
		return api.CreateBudget400JSONResponse{BadRequestJSONResponse: badRequest("budget category must be an expense category")}, nil
	}
	if body.Amount <= 0 {
		return api.CreateBudget400JSONResponse{BadRequestJSONResponse: badRequest("amount must be positive")}, nil
	}

	b := entities.Budget{CategoryID: body.CategoryId, Period: entities.BudgetMonthly, Amount: money.New(body.Amount, s.base)}
	applyBudgetFields(&b, body.Period, body.Rollover, body.StartPeriod)

	if err = s.budgets.Create(ctx, &b); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return api.CreateBudget400JSONResponse{BadRequestJSONResponse: badRequest("a budget already exists for this category")}, nil
		}
		s.logger.Error("create budget", zap.Error(err))

		return nil, err
	}

	resp, err := s.budgetResponse(ctx, b, cat.Name)
	if err != nil {
		return nil, err
	}

	return api.CreateBudget201JSONResponse(resp), nil
}

func (s Server) UpdateBudget(ctx context.Context, request api.UpdateBudgetRequestObject) (api.UpdateBudgetResponseObject, error) {
	b, err := s.budgets.Get(ctx, request.Id)
	if errors.Is(err, budgetrepository.ErrNotFound) {
		return api.UpdateBudget404JSONResponse{NotFoundJSONResponse: notFound("budget not found")}, nil
	}
	if err != nil {
		return nil, err
	}

	body := request.Body
	if body == nil {
		return api.UpdateBudget400JSONResponse{BadRequestJSONResponse: badRequest("empty body")}, nil
	}
	if body.Amount != nil {
		b.Amount = money.New(*body.Amount, s.base)
	}
	applyBudgetFields(b, body.Period, body.Rollover, body.StartPeriod)
	if b.Amount.Minor() <= 0 {
		return api.UpdateBudget400JSONResponse{BadRequestJSONResponse: badRequest("amount must be positive")}, nil
	}

	if err = s.budgets.Update(ctx, b); err != nil {
		s.logger.Error("update budget", zap.Error(err))

		return nil, err
	}

	cat, err := s.categories.Get(ctx, b.CategoryID)
	if err != nil {
		return nil, err
	}
	resp, err := s.budgetResponse(ctx, *b, cat.Name)
	if err != nil {
		return nil, err
	}

	return api.UpdateBudget200JSONResponse(resp), nil
}

func (s Server) DeleteBudget(ctx context.Context, request api.DeleteBudgetRequestObject) (api.DeleteBudgetResponseObject, error) {
	err := s.budgets.Delete(ctx, request.Id)
	if errors.Is(err, budgetrepository.ErrNotFound) {
		return api.DeleteBudget404JSONResponse{NotFoundJSONResponse: notFound("budget not found")}, nil
	}
	if err != nil {
		return nil, err
	}

	return api.DeleteBudget204Response{}, nil
}

func applyBudgetFields(b *entities.Budget, period *api.BudgetPeriod, rollover *bool, startPeriod *openapi_types.Date) {
	if period != nil {
		b.Period = entities.BudgetPeriod(*period)
	}
	if rollover != nil {
		b.Rollover = *rollover
	}
	if startPeriod != nil {
		t := startPeriod.Time
		b.StartPeriod = &t
	}
}

func (s Server) budgetResponse(ctx context.Context, b entities.Budget, categoryName string) (api.Budget, error) {
	start, end := entities.PeriodWindow(b.Period, time.Now())

	spent, err := s.budgets.Spent(ctx, b.CategoryID, start, end)
	if err != nil {
		return api.Budget{}, err
	}

	var percent float32
	if b.Amount.Minor() > 0 {
		percent = float32(float64(spent.Minor()) / float64(b.Amount.Minor()) * 100)
	}

	remaining, err := b.Amount.Minus(spent)
	if err != nil {
		return api.Budget{}, err
	}

	out := api.Budget{
		Id:           b.ID,
		CategoryId:   b.CategoryID,
		CategoryName: categoryName,
		Period:       api.BudgetPeriod(b.Period),
		Amount:       b.Amount,
		Spent:        spent,
		Remaining:    remaining,
		Percent:      percent,
		Rollover:     b.Rollover,
		PeriodStart:  openapi_types.Date{Time: start},
		PeriodEnd:    openapi_types.Date{Time: end.AddDate(0, 0, -1)},
	}
	if b.StartPeriod != nil {
		out.StartPeriod = nullable.NewNullableWithValue(openapi_types.Date{Time: *b.StartPeriod})
	}

	return out, nil
}

func (s Server) categoryNames(ctx context.Context) (map[openapi_types.UUID]string, error) {
	cats, err := s.categories.List(ctx, nil, true)
	if err != nil {
		return nil, err
	}

	names := make(map[openapi_types.UUID]string, len(cats))
	for _, c := range cats {
		names[c.ID] = c.Name
	}

	return names, nil
}
