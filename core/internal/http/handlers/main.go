package handlers

import (
	"context"

	"go.uber.org/zap"

	"finance/generated/api"
)

// Server implements the generated strict OpenAPI interface. Feature handlers
// (accounts, categories, transactions, reports) hang off this type as methods
// in their own files.
type Server struct {
	logger *zap.Logger
}

var _ api.StrictServerInterface = (*Server)(nil)

func NewServer(logger *zap.Logger) *Server {
	return &Server{logger: logger}
}

func (s Server) Health(_ context.Context, _ api.HealthRequestObject) (api.HealthResponseObject, error) {
	return api.Health200JSONResponse{Status: "ok"}, nil
}
