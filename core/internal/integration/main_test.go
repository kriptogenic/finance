// Package integration holds repository tests that run against a real Postgres
// started with testcontainers and migrated with the real migrations. If Docker
// is unavailable the whole package is skipped so `go test ./...` stays green.
package integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/zap"

	"finance/config"
	"finance/internal/entities"
	"finance/internal/ledger"
	accountrepository "finance/internal/repositories/account_repository"
	categoryrepository "finance/internal/repositories/category_repository"
	transactionrepository "finance/internal/repositories/transaction_repository"
	"finance/pkg/database"
	"finance/pkg/migrator"
	"finance/pkg/money"
)

var testDB *database.DB

func TestMain(m *testing.M) {
	ctx := context.Background()

	db, terminate, err := startPostgres(ctx)
	if err != nil {
		// no Docker / can't start a container: skip the suite rather than fail
		fmt.Println("integration tests skipped:", err)
		os.Exit(0)
	}

	testDB = db
	code := m.Run()
	terminate()
	os.Exit(code)
}

func startPostgres(ctx context.Context) (*database.DB, func(), error) {
	container, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("finance_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForListeningPort("5432/tcp").WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		return nil, nil, err
	}

	host, err := container.Host(ctx)
	if err != nil {
		return nil, nil, err
	}
	port, err := container.MappedPort(ctx, "5432/tcp")
	if err != nil {
		return nil, nil, err
	}

	cfg := &config.DB{
		Host:           host,
		Port:           port.Port(),
		User:           "test",
		Password:       "test",
		Name:           "finance_test",
		SSLMode:        "disable",
		MigrationsPath: migrationsDir(),
	}

	logger := zap.NewNop()
	db, err := database.New(cfg, logger)
	if err != nil {
		return nil, nil, err
	}
	if err = db.Connect(ctx); err != nil {
		return nil, nil, err
	}

	mg, err := migrator.New(cfg.MigrationsPath, cfg.MigrationDSN(), logger)
	if err != nil {
		return nil, nil, err
	}
	if err = mg.Up(); err != nil {
		mg.Close()

		return nil, nil, err
	}
	mg.Close()

	terminate := func() {
		_ = db.Close()
		_ = container.Terminate(ctx)
	}

	return db, terminate, nil
}

// migrationsDir resolves core/migrations relative to this test file.
func migrationsDir() string {
	_, file, _, _ := runtime.Caller(0)

	return filepath.Join(filepath.Dir(file), "..", "..", "migrations")
}

// --- shared helpers -------------------------------------------------------

func reset(t *testing.T) {
	t.Helper()
	_, err := testDB.Pool.Exec(context.Background(),
		`TRUNCATE transactions, categories, accounts RESTART IDENTITY CASCADE`)
	require.NoError(t, err)
}

func financeCfg() *config.Finance { return &config.Finance{BaseCurrency: "UZS"} }

func accountRepo() accountrepository.Repository   { return accountrepository.NewRepository(testDB) }
func categoryRepo() categoryrepository.Repository { return categoryrepository.NewRepository(testDB) }
func transactionRepo() transactionrepository.Repository {
	return transactionrepository.NewRepository(testDB, financeCfg())
}

func ptr[T any](v T) *T { return &v }

func mustAccount(t *testing.T, acc entities.Account) entities.Account {
	t.Helper()
	require.NoError(t, accountRepo().Create(context.Background(), &acc))

	return acc
}

func mustCategory(t *testing.T, cat entities.Category) entities.Category {
	t.Helper()
	require.NoError(t, categoryRepo().Create(context.Background(), &cat))

	return cat
}

func assetAccount(currency string, opening int64) entities.Account {
	return entities.Account{
		Name: "acc-" + uuid.NewString()[:8], Kind: entities.KindAsset,
		Type: entities.TypeCash, Currency: currency, OpeningBalance: money.New(opening, currency),
	}
}

func liabilityAccount(currency string, opening int64) entities.Account {
	return entities.Account{
		Name: "liab-" + uuid.NewString()[:8], Kind: entities.KindLiability,
		Type: entities.TypeCreditCard, Currency: currency, OpeningBalance: money.New(opening, currency),
	}
}

func allFilter() transactionrepository.Filter { return transactionrepository.Filter{Limit: 1000} }

// build helpers produce valid transactions via the engine. base == currency so
// no FX rate is required; cross-currency transfers pass the from-leg currency
// as base (its amount is already in base).
func mustBuild(t *testing.T, in ledger.NewTransaction, base string) entities.Transaction {
	t.Helper()
	in.Date = time.Now()
	tx, err := ledger.BuildTransaction(in, base)
	require.NoError(t, err)

	return tx
}

func buildExpense(t *testing.T, from entities.Account, cat entities.Category, amount int64, base string) entities.Transaction {
	return mustBuild(t, ledger.NewTransaction{Type: entities.TxExpense, From: &from, Category: &cat, Amount: money.New(amount, from.Currency)}, base)
}

func buildIncome(t *testing.T, to entities.Account, cat entities.Category, amount int64, base string) entities.Transaction {
	return mustBuild(t, ledger.NewTransaction{Type: entities.TxIncome, To: &to, Category: &cat, Amount: money.New(amount, to.Currency)}, base)
}

func buildTransfer(t *testing.T, from, to entities.Account, amount int64, toAmount *int64, base string) entities.Transaction {
	in := ledger.NewTransaction{Type: entities.TxTransfer, From: &from, To: &to, Amount: money.New(amount, from.Currency)}
	if toAmount != nil {
		m := money.New(*toAmount, to.Currency)
		in.ToAmount = &m
	}

	return mustBuild(t, in, base)
}
