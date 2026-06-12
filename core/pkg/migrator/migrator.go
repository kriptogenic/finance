// Package migrator wraps golang-migrate for use from a standalone CLI command.
// Migrations are never applied implicitly on server start; run cmd/migrate.
package migrator

import (
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5" // registers the "pgx5" database driver
	_ "github.com/golang-migrate/migrate/v4/source/file"     // registers the "file://" source driver

	"go.uber.org/zap"
)

type Migrator struct {
	m      *migrate.Migrate
	path   string
	dsn    string
	logger *zap.Logger
}

// New opens a migrator over the SQL files at migrationsPath using dsn (which
// must carry the "pgx5" scheme, see config.DB.MigrationDSN). Call Close when done.
func New(migrationsPath, dsn string, logger *zap.Logger) (*Migrator, error) {
	mg := &Migrator{path: migrationsPath, dsn: dsn, logger: logger}
	if err := mg.open(); err != nil {
		return nil, err
	}

	return mg, nil
}

func (mg *Migrator) open() error {
	m, err := migrate.New("file://"+mg.path, mg.dsn)
	if err != nil {
		return fmt.Errorf("init migrator: %w", err)
	}

	mg.m = m

	return nil
}

// Up applies all pending migrations; a no-op when already current.
func (mg *Migrator) Up() error {
	if err := mg.m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			mg.logger.Info("schema up to date")

			return nil
		}

		return fmt.Errorf("migrate up: %w", err)
	}

	mg.logVersion()

	return nil
}

// Fresh drops everything and re-applies from scratch. Destructive — local/test use only.
func (mg *Migrator) Fresh() error {
	if err := mg.m.Drop(); err != nil {
		return fmt.Errorf("migrate drop: %w", err)
	}

	mg.logger.Info("schema dropped")

	// Drop deletes schema_migrations but the open driver won't recreate it, so
	// reopen to get a clean version table before re-applying.
	mg.Close()
	if err := mg.open(); err != nil {
		return err
	}

	return mg.Up()
}

func (mg *Migrator) Close() {
	if srcErr, dbErr := mg.m.Close(); srcErr != nil || dbErr != nil {
		mg.logger.Warn("migrator close", zap.NamedError("source", srcErr), zap.NamedError("database", dbErr))
	}
}

func (mg *Migrator) logVersion() {
	version, dirty, err := mg.m.Version()
	if err != nil {
		return
	}

	mg.logger.Info("migrations applied", zap.Uint("version", version), zap.Bool("dirty", dirty))
}
