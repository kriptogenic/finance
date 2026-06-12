package handlers

import (
	"context"

	"go.uber.org/zap"

	"finance/generated/api"
	accountrepository "finance/internal/repositories/account_repository"
	categoryrepository "finance/internal/repositories/category_repository"
)

// Server implements the generated strict OpenAPI interface. Feature handlers
// (accounts, categories, transactions, reports) hang off this type as methods
// in their own files.
type Server struct {
	accounts   accountrepository.Repository
	categories categoryrepository.Repository
	logger     *zap.Logger
}

var _ api.StrictServerInterface = (*Server)(nil)

func NewServer(
	accounts accountrepository.Repository,
	categories categoryrepository.Repository,
	logger *zap.Logger,
) *Server {
	return &Server{accounts: accounts, categories: categories, logger: logger}
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
