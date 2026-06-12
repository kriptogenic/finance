package database

import (
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5" // registers the "pgx5" database driver
	_ "github.com/golang-migrate/migrate/v4/source/file"     // registers the "file://" source driver

	"go.uber.org/zap"
)

// MigrateUp applies all pending migrations. It is a no-op when the schema is
// already current.
func (db *DB) MigrateUp() error {
	m, err := db.migrator()
	if err != nil {
		return err
	}
	defer closeMigrator(m, db.logger)

	if err = m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			db.logger.Info("schema up to date")

			return nil
		}

		return fmt.Errorf("migrate up: %w", err)
	}

	version, dirty, _ := m.Version()
	db.logger.Info("migrations applied", zap.Uint("version", version), zap.Bool("dirty", dirty))

	return nil
}

// MigrateFresh drops everything golang-migrate is aware of and re-applies from
// scratch. Intended for tests and local resets, never production.
func (db *DB) MigrateFresh() error {
	m, err := db.migrator()
	if err != nil {
		return err
	}
	defer closeMigrator(m, db.logger)

	if err = m.Drop(); err != nil {
		return fmt.Errorf("migrate drop: %w", err)
	}

	return db.MigrateUp()
}

func (db *DB) migrator() (*migrate.Migrate, error) {
	src := fmt.Sprintf("file://%s", db.cfg.MigrationsPath)

	m, err := migrate.New(src, db.cfg.MigrationDSN())
	if err != nil {
		return nil, fmt.Errorf("init migrator: %w", err)
	}

	return m, nil
}

func closeMigrator(m *migrate.Migrate, logger *zap.Logger) {
	if srcErr, dbErr := m.Close(); srcErr != nil || dbErr != nil {
		logger.Warn("migrator close", zap.NamedError("source", srcErr), zap.NamedError("database", dbErr))
	}
}
