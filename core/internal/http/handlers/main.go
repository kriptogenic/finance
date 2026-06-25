package handlers

import (
	"context"

	"go.uber.org/zap"

	"finance/config"
	"finance/generated/api"
	"finance/internal/categorysuggest"
	"finance/internal/iconsuggest"
	"finance/internal/ingest"
	accountrepository "finance/internal/repositories/account_repository"
	balancesnapshotrepository "finance/internal/repositories/balance_snapshot_repository"
	budgetrepository "finance/internal/repositories/budget_repository"
	categoryrepository "finance/internal/repositories/category_repository"
	categoryrulerepository "finance/internal/repositories/category_rule_repository"
	pushsubscriptionrepository "finance/internal/repositories/push_subscription_repository"
	reportrepository "finance/internal/repositories/report_repository"
	scheduledtransactionrepository "finance/internal/repositories/scheduled_transaction_repository"
	splitrepository "finance/internal/repositories/split_repository"
	transactionrepository "finance/internal/repositories/transaction_repository"
	"finance/internal/scheduler"
)

// Server implements the generated strict OpenAPI interface. Feature handlers
// (accounts, categories, transactions, reports) hang off this type as methods
// in their own files.
type Server struct {
	accounts       accountrepository.Repository
	categories     categoryrepository.Repository
	categoryRules  categoryrulerepository.Repository
	transactions   transactionrepository.Repository
	splits         splitrepository.Repository
	reports        reportrepository.Repository
	budgets        budgetrepository.Repository
	snapshots      balancesnapshotrepository.Repository
	push           pushsubscriptionrepository.Repository
	schedules      scheduledtransactionrepository.Repository
	materializer   *scheduler.Materializer
	icons          iconsuggest.Suggester
	catSuggest     categorysuggest.Suggester
	ingest         *ingest.Service
	vapidPublicKey string // served to the browser so it can subscribe to push
	base           string // reporting currency (§3)
	logger         *zap.Logger
}

var _ api.StrictServerInterface = (*Server)(nil)

func NewServer(
	accounts accountrepository.Repository,
	categories categoryrepository.Repository,
	categoryRules categoryrulerepository.Repository,
	transactions transactionrepository.Repository,
	splits splitrepository.Repository,
	reports reportrepository.Repository,
	budgets budgetrepository.Repository,
	snapshots balancesnapshotrepository.Repository,
	push pushsubscriptionrepository.Repository,
	schedules scheduledtransactionrepository.Repository,
	materializer *scheduler.Materializer,
	icons iconsuggest.Suggester,
	catSuggest categorysuggest.Suggester,
	ingestSvc *ingest.Service,
	finance *config.Finance,
	pushCfg *config.Push,
	logger *zap.Logger,
) *Server {
	return &Server{
		accounts:       accounts,
		categories:     categories,
		categoryRules:  categoryRules,
		transactions:   transactions,
		splits:         splits,
		reports:        reports,
		budgets:        budgets,
		snapshots:      snapshots,
		push:           push,
		schedules:      schedules,
		materializer:   materializer,
		icons:          icons,
		catSuggest:     catSuggest,
		ingest:         ingestSvc,
		vapidPublicKey: pushCfg.VAPIDPublic,
		base:           finance.BaseCurrency,
		logger:         logger,
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
