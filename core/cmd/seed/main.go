// Command seed populates the database with a realistic dev dataset: a handful of
// asset/liability accounts, expense/income category trees, and a couple of
// months of transactions (including a cross-currency transfer and a frozen FX
// rate). It builds everything through the repositories and the transaction
// engine, so the seeded data obeys every invariant.
//
// Usage:
//
//	go run ./cmd/seed         # seed only when the DB is empty
//	go run ./cmd/seed reset   # wipe accounts/categories/transactions, then seed
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
	"go.uber.org/zap"

	"finance/config"
	"finance/internal/entities"
	"finance/internal/ledger"
	accountrepository "finance/internal/repositories/account_repository"
	balancesnapshotrepository "finance/internal/repositories/balance_snapshot_repository"
	budgetrepository "finance/internal/repositories/budget_repository"
	categoryrepository "finance/internal/repositories/category_repository"
	transactionrepository "finance/internal/repositories/transaction_repository"
	"finance/pkg/database"
	"finance/pkg/fx"
	"finance/pkg/log"
)

func main() {
	if err := godotenv.Load(".env"); err != nil {
		//nolint:forbidigo // logger not configured yet
		fmt.Println("Warning: no .env file found or unable to load it")
	}

	cfg, err := config.NewConfig()
	if err != nil {
		//nolint:forbidigo // logger not configured yet
		fmt.Println("config error:", err)
		os.Exit(1)
	}

	logger := log.NewLogger(&cfg.Log)
	defer func() { _ = logger.Sync() }()

	ctx := context.Background()
	db, err := database.New(&cfg.DB, logger)
	if err != nil {
		logger.Fatal("db init", zap.Error(err))
	}
	if err = db.Connect(ctx); err != nil {
		logger.Fatal("db connect", zap.Error(err))
	}
	defer func() { _ = db.Close() }()

	reset := len(os.Args) > 1 && os.Args[1] == "reset"

	s := &seeder{
		base:       cfg.BaseCurrency,
		accounts:   accountrepository.NewRepository(db),
		categories: categoryrepository.NewRepository(db),
		txns:       transactionrepository.NewRepository(db),
		budgets:    budgetrepository.NewRepository(db),
		snapshots:  balancesnapshotrepository.NewRepository(db),
		logger:     logger,
	}

	if err = s.run(ctx, db, reset); err != nil {
		logger.Fatal("seed", zap.Error(err))
	}
}

type seeder struct {
	base       string
	accounts   accountrepository.Repository
	categories categoryrepository.Repository
	txns       transactionrepository.Repository
	budgets    budgetrepository.Repository
	snapshots  balancesnapshotrepository.Repository
	logger     *zap.Logger
}

func (s *seeder) run(ctx context.Context, db *database.DB, reset bool) error {
	existing, err := s.accounts.List(ctx, true)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		if !reset {
			s.logger.Info("database already has data; pass 'reset' to reseed", zap.Int("accounts", len(existing)))

			return nil
		}
		if _, err = db.Pool.Exec(ctx,
			`TRUNCATE transactions, categories, accounts, balance_snapshots RESTART IDENTITY CASCADE`); err != nil {
			return fmt.Errorf("truncate: %w", err)
		}
		s.logger.Info("existing data wiped")
	}

	accounts, err := s.seedAccounts(ctx)
	if err != nil {
		return err
	}

	cats, err := s.seedCategories(ctx)
	if err != nil {
		return err
	}

	if err = s.seedTransactions(ctx, accounts, cats); err != nil {
		return err
	}

	if err = s.seedBudgets(ctx, cats); err != nil {
		return err
	}

	if err = s.seedSnapshots(ctx); err != nil {
		return err
	}

	s.logger.Info("seed complete",
		zap.Int("accounts", len(accounts)), zap.Int("categories", len(cats)))

	return nil
}

func (s *seeder) seedBudgets(ctx context.Context, cat map[string]entities.Category) error {
	defs := []entities.Budget{
		{CategoryID: cat["Food"].ID, Period: entities.BudgetMonthly, Amount: 200_000_00},
		{CategoryID: cat["Rent"].ID, Period: entities.BudgetMonthly, Amount: 2_000_000_00},
	}
	for i := range defs {
		b := defs[i]
		if err := s.budgets.Create(ctx, &b); err != nil {
			return fmt.Errorf("seed budget: %w", err)
		}
	}

	return nil
}

func ptr[T any](v T) *T { return &v }

// seedSnapshots seeds reported card balances for the Humo cards so the
// reconciliation tab has data: *8400 matches its derived balance, *4853 is off.
func (s *seeder) seedSnapshots(ctx context.Context) error {
	now := time.Now()
	defs := []entities.BalanceSnapshot{
		{CardLast4: "8400", Bank: ptr("TBCBANK"), Amount: 1_100_000_00, Currency: "UZS", Source: ptr("humo"), ReportedAt: now},
		{CardLast4: "4853", Bank: ptr("IPAKYULIBANK"), Amount: 700_050_00, Currency: "UZS", Source: ptr("humo"), ReportedAt: now},
	}
	for i := range defs {
		snap := defs[i]
		if err := s.snapshots.Upsert(ctx, &snap); err != nil {
			return fmt.Errorf("seed snapshot %q: %w", snap.CardLast4, err)
		}
	}

	return nil
}

func (s *seeder) seedAccounts(ctx context.Context) (map[string]entities.Account, error) {
	defs := []entities.Account{
		{Name: "Cash", Kind: entities.KindAsset, Type: entities.TypeCash, Currency: "UZS", OpeningBalance: 2_000_000_00},
		{Name: "Humo *4853", Kind: entities.KindAsset, Type: entities.TypeDebitCard, Currency: "UZS", OpeningBalance: 700_000_00, CardLast4: ptr("4853")},
		{Name: "Humo *8400", Kind: entities.KindAsset, Type: entities.TypeDebitCard, Currency: "UZS", OpeningBalance: 1_100_000_00, CardLast4: ptr("8400")},
		{Name: "USD Wallet", Kind: entities.KindAsset, Type: entities.TypeCash, Currency: "USD", OpeningBalance: 0},
		{
			Name: "Savings", Kind: entities.KindAsset, Type: entities.TypeDeposit, Currency: "UZS",
			OpeningBalance: 5_000_000_00, InterestRate: ptr(0.18), TermMonths: ptr(12),
		},
		{
			Name: "Visa Card", Kind: entities.KindLiability, Type: entities.TypeCreditCard, Currency: "UZS",
			OpeningBalance: 0, CreditLimit: ptr(int64(10_000_000_00)),
		},
		{
			Name: "Home Loan", Kind: entities.KindLiability, Type: entities.TypeLoan, Currency: "UZS",
			OpeningBalance: 50_000_000_00, Principal: ptr(int64(60_000_000_00)),
			InterestRate: ptr(0.16), TermMonths: ptr(240), PaymentDay: ptr(5),
			StartDate: ptr(time.Date(2025, time.June, 5, 0, 0, 0, 0, time.UTC)),
		},
	}

	out := make(map[string]entities.Account, len(defs))
	for i := range defs {
		acc := defs[i]
		if err := s.accounts.Create(ctx, &acc); err != nil {
			return nil, fmt.Errorf("seed account %q: %w", acc.Name, err)
		}
		out[acc.Name] = acc
	}

	return out, nil
}

// catDef is a category to seed; icon is a Tabler icon name and color a hex from
// the frontend palette, so seeded categories render with the same look as ones
// created in the UI.
type catDef struct {
	name  string
	icon  string
	color string
}

func (s *seeder) seedCategories(ctx context.Context) (map[string]entities.Category, error) {
	out := make(map[string]entities.Category)

	create := func(def catDef, typ entities.CategoryType, parent *entities.Category) (entities.Category, error) {
		c := entities.Category{Name: def.name, Type: typ, Icon: ptr(def.icon), Color: ptr(def.color)}
		if parent != nil {
			c.ParentID = &parent.ID
		}
		if err := s.categories.Create(ctx, &c); err != nil {
			return entities.Category{}, fmt.Errorf("seed category %q: %w", def.name, err)
		}
		out[def.name] = c

		return c, nil
	}

	food, err := create(catDef{"Food", "meat", "#f97316"}, entities.CategoryExpense, nil)
	if err != nil {
		return nil, err
	}
	for _, def := range []catDef{
		{"Groceries", "shopping-cart", "#22c55e"},
		{"Restaurants", "tools-kitchen-2", "#f59e0b"},
	} {
		if _, err = create(def, entities.CategoryExpense, &food); err != nil {
			return nil, err
		}
	}
	for _, def := range []catDef{
		{"Rent", "home", "#3b82f6"},
		{"Transport", "car", "#06b6d4"},
		{"Utilities", "bolt", "#eab308"},
	} {
		if _, err = create(def, entities.CategoryExpense, nil); err != nil {
			return nil, err
		}
	}
	for _, def := range []catDef{
		{"Salary", "cash", "#10b981"},
		{"Freelance", "briefcase", "#8b5cf6"},
	} {
		if _, err = create(def, entities.CategoryIncome, nil); err != nil {
			return nil, err
		}
	}
	// "Uncategorized" buckets for externally-ingested transactions; tagged with a
	// system_key so the ingest endpoint can find them as the default category.
	if err = s.createSystemCategory(ctx, out, catDef{"Uncategorized", "help-circle", "#64748b"}, entities.CategoryExpense, "uncategorized_expense"); err != nil {
		return nil, err
	}
	if err = s.createSystemCategory(ctx, out, catDef{"Uncategorized income", "help-circle", "#64748b"}, entities.CategoryIncome, "uncategorized_income"); err != nil {
		return nil, err
	}

	return out, nil
}

func (s *seeder) createSystemCategory(ctx context.Context, out map[string]entities.Category, def catDef, typ entities.CategoryType, key string) error {
	c := entities.Category{Name: def.name, Type: typ, Icon: ptr(def.icon), Color: ptr(def.color), SystemKey: &key}
	if err := s.categories.Create(ctx, &c); err != nil {
		return fmt.Errorf("seed system category %q: %w", def.name, err)
	}
	out[def.name] = c

	return nil
}

func (s *seeder) seedTransactions(ctx context.Context, acc map[string]entities.Account, cat map[string]entities.Category) error {
	day := func(d int, month time.Month) time.Time {
		return time.Date(2026, month, d, 12, 0, 0, 0, time.UTC)
	}
	usdRate := fx.MustParseRate("12500")

	type spec struct {
		date time.Time
		in   ledger.NewTransaction
	}

	from := func(name string) *entities.Account { a := acc[name]; return &a }
	category := func(name string) *entities.Category { c := cat[name]; return &c }

	specs := []spec{
		{day(1, time.May), ledger.NewTransaction{Type: entities.TxIncome, To: from("Cash"), Category: category("Salary"), Amount: 5_000_000_00}},
		{day(3, time.May), ledger.NewTransaction{Type: entities.TxExpense, From: from("Cash"), Category: category("Rent"), Amount: 1_500_000_00}},
		{day(10, time.May), ledger.NewTransaction{Type: entities.TxExpense, From: from("Cash"), Category: category("Groceries"), Amount: 300_000_00, Tags: []string{"weekly"}}},
		{day(15, time.May), ledger.NewTransaction{Type: entities.TxExpense, From: from("Cash"), Category: category("Transport"), Amount: 50_000_00}},
		{day(1, time.June), ledger.NewTransaction{Type: entities.TxIncome, To: from("Cash"), Category: category("Salary"), Amount: 5_000_000_00}},
		{day(5, time.June), ledger.NewTransaction{Type: entities.TxExpense, From: from("Visa Card"), Category: category("Restaurants"), Amount: 120_000_00, Note: ptr("dinner")}},
		// cross-currency transfer: 125,000.00 UZS -> 10.00 USD
		{day(6, time.June), ledger.NewTransaction{Type: entities.TxTransfer, From: from("Cash"), To: from("USD Wallet"), Amount: 125_000_00, ToAmount: ptr(int64(10_00))}},
		// USD expense with a frozen rate to base (UZS)
		{day(7, time.June), ledger.NewTransaction{Type: entities.TxExpense, From: from("USD Wallet"), Category: category("Food"), Amount: 5_00, RateToBase: &usdRate}},
		// repay part of the card bill
		{day(10, time.June), ledger.NewTransaction{Type: entities.TxTransfer, From: from("Cash"), To: from("Visa Card"), Amount: 200_000_00}},
	}

	for _, sp := range specs {
		in := sp.in
		in.Date = sp.date

		tx, err := ledger.BuildTransaction(in, s.base)
		if err != nil {
			return fmt.Errorf("build seed transaction (%s): %w", in.Type, err)
		}
		if err = s.txns.Create(ctx, &tx); err != nil {
			return fmt.Errorf("create seed transaction (%s): %w", in.Type, err)
		}
	}

	return nil
}
