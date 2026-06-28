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

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"go.uber.org/zap"

	"finance/config"
	"finance/internal/entities"
	"finance/internal/ledger"
	accountrepository "finance/internal/repositories/account_repository"
	balancesnapshotrepository "finance/internal/repositories/balance_snapshot_repository"
	budgetrepository "finance/internal/repositories/budget_repository"
	categoryrepository "finance/internal/repositories/category_repository"
	receiptrepository "finance/internal/repositories/receipt_repository"
	scheduledtransactionrepository "finance/internal/repositories/scheduled_transaction_repository"
	transactionrepository "finance/internal/repositories/transaction_repository"
	"finance/pkg/database"
	"finance/pkg/fx"
	"finance/pkg/log"
	"finance/pkg/money"
)

func uzs(v int64) money.Money { return money.New(v, "UZS") }
func usd(v int64) money.Money { return money.New(v, "USD") }

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
		budgets:    budgetrepository.NewRepository(db, &cfg.Finance),
		snapshots:  balancesnapshotrepository.NewRepository(db),
		receipts:   receiptrepository.NewRepository(db),
		schedules:  scheduledtransactionrepository.NewRepository(db),
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
	receipts   receiptrepository.Repository
	schedules  scheduledtransactionrepository.Repository
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
			`TRUNCATE transactions, scheduled_transactions, categories, accounts, balance_snapshots, receipts RESTART IDENTITY CASCADE`); err != nil {
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

	txns, err := s.seedTransactions(ctx, accounts, cats)
	if err != nil {
		return err
	}

	if err = s.seedReceipts(ctx, txns); err != nil {
		return err
	}

	if err = s.seedBudgets(ctx, cats); err != nil {
		return err
	}

	if err = s.seedSchedules(ctx, accounts, cats); err != nil {
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
		{CategoryID: cat["Food"].ID, Period: entities.BudgetMonthly, Amount: uzs(200_000_00)},
		{CategoryID: cat["Rent"].ID, Period: entities.BudgetMonthly, Amount: uzs(2_000_000_00)},
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

// seedSchedules creates recurring plan items (salary in; rent, loan, card top-up
// out) that drive the forecast. Anchored early in 2026 and monthly, so they
// recur into the current month. Loan/card payments are transfers (planned
// outflow); salary/rent carry a matching category.
func (s *seeder) seedSchedules(ctx context.Context, acc map[string]entities.Account, cat map[string]entities.Category) error {
	day := func(d int) time.Time { return time.Date(2026, time.January, d, 0, 0, 0, 0, time.UTC) }
	accID := func(name string) *uuid.UUID { a := acc[name]; return &a.ID }
	catID := func(name string) *uuid.UUID { c := cat[name]; return &c.ID }

	defs := []entities.ScheduledTransaction{
		{
			Name: ptr("Salary"), Type: entities.TxIncome, ToAccountID: accID("Cash"), CategoryID: catID("Salary"),
			Amount: uzs(5_000_000_00), Frequency: entities.FreqMonthly, Interval: 1, StartDate: day(1),
		},
		{
			Name: ptr("Rent"), Type: entities.TxExpense, FromAccountID: accID("Cash"), CategoryID: catID("Rent"),
			Amount: uzs(1_500_000_00), Frequency: entities.FreqMonthly, Interval: 1, StartDate: day(3),
		},
		{
			Name: ptr("Home loan payment"), Type: entities.TxTransfer, FromAccountID: accID("Cash"), ToAccountID: accID("Home Loan"),
			Amount: uzs(900_000_00), Frequency: entities.FreqMonthly, Interval: 1, StartDate: day(5),
		},
		{
			Name: ptr("Visa Card top-up"), Type: entities.TxTransfer, FromAccountID: accID("Cash"), ToAccountID: accID("Visa Card"),
			Amount: uzs(300_000_00), Frequency: entities.FreqMonthly, Interval: 1, StartDate: day(10),
		},
	}

	for i := range defs {
		sc := defs[i]
		if err := s.schedules.Create(ctx, &sc); err != nil {
			return fmt.Errorf("seed schedule %q: %w", *sc.Name, err)
		}
	}

	return nil
}

// seedSnapshots seeds reported card balances for the Humo cards so the
// reconciliation tab has data: *8400 matches its derived balance, *4853 is off.
func (s *seeder) seedSnapshots(ctx context.Context) error {
	now := time.Now()
	defs := []entities.BalanceSnapshot{
		{CardLast4: "8400", Bank: ptr("TBCBANK"), Amount: uzs(1_100_000_00), Source: ptr("humo"), ReportedAt: now},
		{CardLast4: "4853", Bank: ptr("IPAKYULIBANK"), Amount: uzs(700_050_00), Source: ptr("humo"), ReportedAt: now},
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
		{Name: "Cash", Kind: entities.KindAsset, Type: entities.TypeCash, Currency: "UZS", OpeningBalance: uzs(2_000_000_00)},
		{Name: "Humo *4853", Kind: entities.KindAsset, Type: entities.TypeDebitCard, Currency: "UZS", OpeningBalance: uzs(700_000_00), CardLast4: ptr("4853")},
		{Name: "Humo *8400", Kind: entities.KindAsset, Type: entities.TypeDebitCard, Currency: "UZS", OpeningBalance: uzs(1_100_000_00), CardLast4: ptr("8400")},
		{Name: "USD Wallet", Kind: entities.KindAsset, Type: entities.TypeCash, Currency: "USD", OpeningBalance: usd(0)},
		{
			Name: "Savings", Kind: entities.KindAsset, Type: entities.TypeDeposit, Currency: "UZS",
			OpeningBalance: uzs(5_000_000_00), InterestRate: ptr(0.18), TermMonths: ptr(12),
		},
		{
			Name: "Visa Card", Kind: entities.KindLiability, Type: entities.TypeCreditCard, Currency: "UZS",
			OpeningBalance: uzs(0), CreditLimit: ptr(uzs(10_000_000_00)),
		},
		{
			Name: "Home Loan", Kind: entities.KindLiability, Type: entities.TypeLoan, Currency: "UZS",
			OpeningBalance: uzs(50_000_000_00), Principal: ptr(uzs(60_000_000_00)),
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

// seedTransactions creates the dev transactions and returns the ones tagged with
// a label, so later steps (e.g. receipts) can link to a specific transaction.
func (s *seeder) seedTransactions(ctx context.Context, acc map[string]entities.Account, cat map[string]entities.Category) (map[string]entities.Transaction, error) {
	day := func(d int, month time.Month) time.Time {
		return time.Date(2026, month, d, 12, 0, 0, 0, time.UTC)
	}
	usdRate := fx.MustParseRate("12500")

	type spec struct {
		label string // non-empty labels are returned for cross-referencing
		date  time.Time
		in    ledger.NewTransaction
	}

	from := func(name string) *entities.Account { a := acc[name]; return &a }
	category := func(name string) *entities.Category { c := cat[name]; return &c }

	specs := []spec{
		{"", day(1, time.May), ledger.NewTransaction{Type: entities.TxIncome, To: from("Cash"), Category: category("Salary"), Amount: uzs(5_000_000_00)}},
		{"", day(3, time.May), ledger.NewTransaction{Type: entities.TxExpense, From: from("Cash"), Category: category("Rent"), Amount: uzs(1_500_000_00)}},
		{"groceries", day(10, time.May), ledger.NewTransaction{Type: entities.TxExpense, From: from("Cash"), Category: category("Groceries"), Amount: uzs(300_000_00), Tags: []string{"weekly"}}},
		{"", day(15, time.May), ledger.NewTransaction{Type: entities.TxExpense, From: from("Cash"), Category: category("Transport"), Amount: uzs(50_000_00)}},
		{"", day(1, time.June), ledger.NewTransaction{Type: entities.TxIncome, To: from("Cash"), Category: category("Salary"), Amount: uzs(5_000_000_00)}},
		{"dinner", day(5, time.June), ledger.NewTransaction{Type: entities.TxExpense, From: from("Visa Card"), Category: category("Restaurants"), Amount: uzs(120_000_00), Note: ptr("dinner")}},
		// cross-currency transfer: 125,000.00 UZS -> 10.00 USD
		{"", day(6, time.June), ledger.NewTransaction{Type: entities.TxTransfer, From: from("Cash"), To: from("USD Wallet"), Amount: uzs(125_000_00), ToAmount: ptr(usd(10_00))}},
		// USD expense with a frozen rate to base (UZS)
		{"", day(7, time.June), ledger.NewTransaction{Type: entities.TxExpense, From: from("USD Wallet"), Category: category("Food"), Amount: usd(5_00), RateToBase: &usdRate}},
		// repay part of the card bill
		{"", day(10, time.June), ledger.NewTransaction{Type: entities.TxTransfer, From: from("Cash"), To: from("Visa Card"), Amount: uzs(200_000_00)}},
	}

	out := make(map[string]entities.Transaction)
	for _, sp := range specs {
		in := sp.in
		in.Date = sp.date

		tx, err := ledger.BuildTransaction(in, s.base)
		if err != nil {
			return nil, fmt.Errorf("build seed transaction (%s): %w", in.Type, err)
		}
		if err = s.txns.Create(ctx, &tx); err != nil {
			return nil, fmt.Errorf("create seed transaction (%s): %w", in.Type, err)
		}
		if sp.label != "" {
			out[sp.label] = tx
		}
	}

	return out, nil
}

// seedReceipt describes a fully-parsed fiscal receipt to seed. total is derived
// from paidCard+paidCash; linkLabel ties it to a labeled seed transaction.
type seedReceipt struct {
	qrURL      string
	terminal   string
	seq        int
	sign       string
	merchant   string
	tin        string
	address    string
	cardType   string
	receivedAt time.Time
	paidCard   int64
	paidCash   int64
	items      []entities.ReceiptItem
	linkLabel  string
}

func (s *seeder) seedReceipts(ctx context.Context, txns map[string]entities.Transaction) error {
	// item builds a line with 12% VAT extracted from the (VAT-inclusive) price.
	item := func(name, qty string, price int64) entities.ReceiptItem {
		return entities.ReceiptItem{
			Name: name, Quantity: qty, Price: uzs(price), VATAmount: uzs(price * 12 / 112), VATRate: 12,
			Discount: uzs(0), Other: uzs(0),
		}
	}

	defs := []seedReceipt{
		{
			qrURL: "https://ofd.soliq.uz/check?t=UZ123456789&r=000123&s=987654321098", linkLabel: "groceries",
			terminal: "UZ123456789", seq: 123, sign: "987654321098",
			merchant: "Korzinka Yunusobod", tin: "302345678", address: "Toshkent, Yunusobod 19",
			cardType: "UZCARD", receivedAt: txns["groceries"].Date, paidCard: 300_000_00,
			items: []entities.ReceiptItem{
				item("Sut 2.5% 1L", "2", 30_000_00),
				item("Non", "4", 20_000_00),
				item("Tovuq filesi", "1.2", 150_000_00),
				item("Olma", "3", 100_000_00),
			},
		},
		{
			qrURL: "https://ofd.soliq.uz/check?t=UZ555000111&r=004521&s=445566778899", linkLabel: "dinner",
			terminal: "UZ555000111", seq: 4521, sign: "445566778899",
			merchant: "Caffe Bon", tin: "301889900", address: "Toshkent, Amir Temur 12",
			cardType: "HUMO", receivedAt: txns["dinner"].Date, paidCard: 120_000_00,
			items: []entities.ReceiptItem{
				item("Lavash", "2", 70_000_00),
				item("Choy", "2", 20_000_00),
				item("Shirinlik", "1", 30_000_00),
			},
		},
		{
			// not linked to any transaction — shows the unlinked state
			qrURL:    "https://ofd.soliq.uz/check?t=UZ777222333&r=000088&s=112233445566",
			terminal: "UZ777222333", seq: 88, sign: "112233445566",
			merchant: "Dori-Darmon", tin: "300112233", address: "Toshkent, Chilonzor 5",
			cardType: "CASH", receivedAt: time.Date(2026, time.June, 12, 18, 30, 0, 0, time.UTC), paidCash: 45_000_00,
			items: []entities.ReceiptItem{
				item("Paratsetamol", "1", 15_000_00),
				item("Vitamin C", "2", 30_000_00),
			},
		},
	}

	for _, def := range defs {
		if err := s.parsedReceipt(ctx, def, txns); err != nil {
			return err
		}
	}

	// a receipt whose scrape failed — exercises the failed status in the UI
	failed := entities.Receipt{
		QRURL: "https://ofd.soliq.uz/check?t=UZ909090909&r=000042&s=778899001122", Status: entities.ReceiptPending,
		TerminalID: ptr("UZ909090909"), ReceiptSeq: ptr(42), FiscalSign: ptr("778899001122"),
		ReceivedAt: ptr(time.Date(2026, time.June, 14, 9, 15, 0, 0, time.UTC)),
	}
	if err := s.receipts.Create(ctx, &failed); err != nil {
		return fmt.Errorf("create failed receipt: %w", err)
	}
	if err := s.receipts.SetStatus(ctx, failed.ID, entities.ReceiptFailed, ptr("fetch receipt: proxy unavailable")); err != nil {
		return fmt.Errorf("mark receipt failed: %w", err)
	}

	return nil
}

// parsedReceipt inserts a receipt header, saves its parsed fields + items, and
// links it to a seed transaction when linkLabel is set.
func (s *seeder) parsedReceipt(ctx context.Context, def seedReceipt, txns map[string]entities.Transaction) error {
	rec := entities.Receipt{
		QRURL: def.qrURL, Status: entities.ReceiptPending,
		TerminalID: &def.terminal, ReceiptSeq: &def.seq, FiscalSign: &def.sign, ReceivedAt: &def.receivedAt,
	}
	if err := s.receipts.Create(ctx, &rec); err != nil {
		return fmt.Errorf("create receipt %q: %w", def.merchant, err)
	}

	var vat int64
	for _, it := range def.items {
		vat += it.VATAmount.Minor()
	}
	rec.ReceiptType = ptr("Sale")
	rec.MerchantName = &def.merchant
	rec.MerchantTIN = &def.tin
	rec.MerchantAddress = &def.address
	rec.CardType = &def.cardType
	rec.PaidCard = uzs(def.paidCard)
	rec.PaidCash = uzs(def.paidCash)
	rec.TotalAmount = uzs(def.paidCard + def.paidCash)
	rec.TotalVAT = uzs(vat)
	rec.Items = def.items
	if err := s.receipts.SaveParsed(ctx, &rec); err != nil {
		return fmt.Errorf("save parsed receipt %q: %w", def.merchant, err)
	}

	if def.linkLabel != "" {
		tx := txns[def.linkLabel]
		if err := s.receipts.SetTransaction(ctx, rec.ID, &tx.ID); err != nil {
			return fmt.Errorf("link receipt %q: %w", def.merchant, err)
		}
	}

	return nil
}
