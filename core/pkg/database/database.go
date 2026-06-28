package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"finance/config"
	"finance/pkg/money"
)

type DB struct {
	Pool   *pgxpool.Pool
	cfg    *config.DB
	logger *zap.Logger
}

func New(cfg *config.DB, logger *zap.Logger) (*DB, error) {
	return &DB{
		cfg:    cfg,
		logger: logger,
	}, nil
}

func (db *DB) Connect(ctx context.Context) error {
	cfg, err := pgxpool.ParseConfig(db.cfg.DSN())
	if err != nil {
		return fmt.Errorf("parse database config: %w", err)
	}
	// register the money_t composite codec on every pooled connection (the type
	// must already exist — run migrations first).
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		return money.RegisterType(ctx, conn)
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}

	var version string
	if err = pool.QueryRow(ctx, "SELECT version()").Scan(&version); err != nil {
		pool.Close()
		return fmt.Errorf("ping database: %w", err)
	}

	db.logger.Info("connected to database", zap.String("version", version))
	db.Pool = pool

	return nil
}

func (db *DB) Close() error {
	if db.Pool != nil {
		db.Pool.Close()
	}

	return nil
}
