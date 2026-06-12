package app

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap"

	"finance/config"
	"finance/internal/http/handlers"
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
			handlers.NewServer,
			httpHandler,
		),
		fx.Invoke(
			dbLifecycle,
			HTTPLifecycle,
		),
	)
}

func httpHandler(server *handlers.Server, cfg *config.HTTP) http.Handler {
	r := chi.NewMux()
	r.Use(middleware.Recoverer)

	if cfg.CORSOrigin != "" {
		r.Use(cors.Handler(cors.Options{
			AllowedOrigins: []string{cfg.CORSOrigin},
			AllowedMethods: []string{"GET", "POST", "PATCH", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders: []string{"Authorization", "Content-Type"},
		}))
	}

	r.Get("/health", server.Health)

	return r
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
