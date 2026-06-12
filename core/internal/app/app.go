package app

import (
	"context"
	"net/http"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap"

	"finance/config"
	"finance/generated/api"
	"finance/internal/http/handlers"
	"finance/internal/http/middlewares"
	accountrepository "finance/internal/repositories/account_repository"
	categoryrepository "finance/internal/repositories/category_repository"
	"finance/pkg/database"
	"finance/pkg/httpserver"
	"finance/pkg/log"
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
			handlers.NewServer,
			httpHandler,
		),
		fx.Invoke(
			dbLifecycle,
			HTTPLifecycle,
		),
	)
}

func httpHandler(server *handlers.Server, spec *openapi3.T, cfg *config.HTTP) http.Handler {
	r := chi.NewMux()
	r.Use(middleware.Recoverer)

	if cfg.CORSOrigin != "" {
		r.Use(cors.Handler(cors.Options{
			AllowedOrigins: []string{cfg.CORSOrigin},
			AllowedMethods: []string{"GET", "POST", "PATCH", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders: []string{"Authorization", "Content-Type"},
		}))
	}

	strictServer := api.NewStrictHandler(server, nil)

	return api.HandlerWithOptions(strictServer, api.ChiServerOptions{
		BaseRouter: r,
		Middlewares: []api.MiddlewareFunc{
			middlewares.OpenAPIRequestValidator(spec),
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
				OnStop: func(context.Context) error { return logger.Sync() },
			})
		}),
	)
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
