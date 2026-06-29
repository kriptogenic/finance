package app

import (
	"context"
	"errors"
	"finance/pkg/proxy"
	"net/http"
	"syscall"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap"

	"finance/config"
	"finance/generated/api"
	"finance/internal/categorysuggest"
	"finance/internal/http/handlers"
	"finance/internal/http/middlewares"
	"finance/internal/iconsuggest"
	"finance/internal/ingest"
	"finance/internal/pushnotify"
	"finance/internal/receipts"
	accountrepository "finance/internal/repositories/account_repository"
	balancesnapshotrepository "finance/internal/repositories/balance_snapshot_repository"
	budgetrepository "finance/internal/repositories/budget_repository"
	categoryrepository "finance/internal/repositories/category_repository"
	categoryrulerepository "finance/internal/repositories/category_rule_repository"
	pushsubscriptionrepository "finance/internal/repositories/push_subscription_repository"
	receiptrepository "finance/internal/repositories/receipt_repository"
	reportrepository "finance/internal/repositories/report_repository"
	scheduledtransactionrepository "finance/internal/repositories/scheduled_transaction_repository"
	splitrepository "finance/internal/repositories/split_repository"
	transactionrepository "finance/internal/repositories/transaction_repository"
	"finance/pkg/database"
	"finance/pkg/httpserver"
	"finance/pkg/log"
	"finance/pkg/s3"
)

func CreateApp() fx.Option {
	return fx.Options(
		config.RegisterConfigs(),
		Logger(log.NewLogger),
		fx.Provide(
			database.New,
			swagger,
			accountrepository.NewRepository,
			categoryrepository.NewRepository,
			categoryrulerepository.NewRepository,
			transactionrepository.NewRepository,
			splitrepository.NewRepository,
			reportrepository.NewRepository,
			budgetrepository.NewRepository,
			balancesnapshotrepository.NewRepository,
			pushsubscriptionrepository.NewRepository,
			scheduledtransactionrepository.NewRepository,
			receiptrepository.NewRepository,
			newS3,
			newProxy,
			receipts.NewService,
			iconsuggest.New,
			categorysuggest.New,
			pushnotify.New,
			ingest.NewService,
			handlers.NewServer,
			httpHandler,
		),
		fx.Invoke(
			dbLifecycle,
			HTTPLifecycle,
			receipts.Lifecycle,
		),
	)
}

func newS3(cfg *config.S3) *s3.Client {
	return s3.New(s3.Config{
		Endpoint:        cfg.Endpoint,
		Region:          cfg.Region,
		Bucket:          cfg.Bucket,
		AccessKeyID:     cfg.AccessKeyID,
		SecretAccessKey: cfg.SecretAccessKey,
	})
}

func newProxy(cfg *config.Proxy) *proxy.Client {
	return proxy.New(cfg.URL, cfg.Secret)
}

func httpHandler(server *handlers.Server, spec *openapi3.T, cfg *config.Config, logger *zap.Logger) http.Handler {
	r := chi.NewMux()
	r.Use(middleware.Recoverer)

	if cfg.HTTP.CORSOrigin != "" {
		r.Use(cors.Handler(cors.Options{
			AllowedOrigins: []string{cfg.HTTP.CORSOrigin},
			AllowedMethods: []string{"GET", "POST", "PATCH", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders: []string{"Authorization", "Content-Type"},
		}))
	}

	r.Use(middlewares.AuthRateLimiter(cfg.RateLimit.AuthAttempts, cfg.RateLimit.AuthWindow))

	strictServer := api.NewStrictHandler(server, []api.StrictMiddlewareFunc{
		middlewares.IngestRequestLogger(logger),
	})

	return api.HandlerWithOptions(strictServer, api.ChiServerOptions{
		BaseRouter: r,
		Middlewares: []api.MiddlewareFunc{
			middlewares.OpenAPIRequestValidator(spec, &cfg.Auth, &cfg.Ingest),
		},
	})
}

// swagger returns the embedded OpenAPI spec used by the request validator.
func swagger() (*openapi3.T, error) {
	spec, err := api.GetSpec()
	if err != nil {
		return nil, err
	}
	spec.Servers = nil

	return spec, nil
}

func Logger(loggerFactory func(cfg *config.Log) *zap.Logger) fx.Option {
	return fx.Options(
		fx.Provide(loggerFactory),
		fx.WithLogger(func(logger *zap.Logger) fxevent.Logger {
			return &fxevent.ZapLogger{Logger: logger}
		}),
		fx.Invoke(func(logger *zap.Logger, lc fx.Lifecycle) {
			lc.Append(fx.Hook{
				OnStop: func(context.Context) error {
					if err := logger.Sync(); err != nil && !isSyncStderrErr(err) {
						return err
					}
					return nil
				},
			})
		}),
	)
}

// isSyncStderrErr reports whether err is the benign error returned when syncing
// stdout/stderr (a pipe or character device under Docker/k8s).
func isSyncStderrErr(err error) bool {
	return errors.Is(err, syscall.EINVAL) || errors.Is(err, syscall.ENOTTY)
}

func dbLifecycle(lc fx.Lifecycle, db *database.DB) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return db.Connect(ctx)
		},
		OnStop: func(context.Context) error {
			return db.Close()
		},
	})
}

func HTTPLifecycle(lc fx.Lifecycle, handler http.Handler, cfg *config.HTTP) {
	server := httpserver.New(handler, httpserver.Port(cfg.Port))
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return server.Start(ctx)
		},
		OnStop: func(context.Context) error {
			return server.Shutdown()
		},
	})
}
