package handlers

import (
	"context"

	"go.uber.org/zap"

	"finance/config"
	"finance/generated/api"
	accountrepository "finance/internal/repositories/account_repository"
	budgetrepository "finance/internal/repositories/budget_repository"
	categoryrepository "finance/internal/repositories/category_repository"
	reportrepository "finance/internal/repositories/report_repository"
	transactionrepository "finance/internal/repositories/transaction_repository"
)

// Server implements the generated strict OpenAPI interface. Feature handlers
// (accounts, categories, transactions, reports) hang off this type as methods
// in their own files.
type Server struct {
	accounts     accountrepository.Repository
	categories   categoryrepository.Repository
	transactions transactionrepository.Repository
	reports      reportrepository.Repository
	budgets      budgetrepository.Repository
	base         string // reporting currency (§3)
	logger       *zap.Logger
}

var _ api.StrictServerInterface = (*Server)(nil)

func NewServer(
	accounts accountrepository.Repository,
	categories categoryrepository.Repository,
	transactions transactionrepository.Repository,
	reports reportrepository.Repository,
	budgets budgetrepository.Repository,
	finance *config.Finance,
	logger *zap.Logger,
) *Server {
	return &Server{
		accounts:     accounts,
		categories:   categories,
		transactions: transactions,
		reports:      reports,
		budgets:      budgets,
		base:         finance.BaseCurrency,
		logger:       logger,
	}
}

func (s Server) Health(_ context.Context, _ api.HealthRequestObject) (api.HealthResponseObject, error) {
	return api.Health200JSONResponse{Status: "ok"}, nil
}

func badRequest(msg string) api.BadRequestJSONResponse {
	return api.BadRequestJSONResponse{Error: msg}
}

func notFound(msg string) api.NotFoundJSONResponse {
	return api.NotFoundJSONResponse{Error: msg}
}
