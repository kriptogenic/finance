package audit

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"finance/internal/entities"
	"finance/internal/ledger"
	transactionrepository "finance/internal/repositories/transaction_repository"
	mcpwrap "finance/pkg/mcp"
	"finance/pkg/money"
)

// register wires every audit tool onto the MCP server.
func (s *Service) register(srv *mcpwrap.Server) {
	mcpwrap.AddTool(srv, &mcpwrap.Tool{
		Name:        "net_worth",
		Description: "Current net worth: assets minus liabilities in the base currency, with a per-currency breakdown and any currencies missing an exchange rate.",
	}, s.netWorth)

	mcpwrap.AddTool(srv, &mcpwrap.Tool{
		Name:        "list_accounts",
		Description: "List accounts (cash, cards, loans, deposits, receivables) with their derived balances.",
	}, s.listAccounts)

	mcpwrap.AddTool(srv, &mcpwrap.Tool{
		Name:        "list_categories",
		Description: "List income/expense categories and subcategories.",
	}, s.listCategories)

	mcpwrap.AddTool(srv, &mcpwrap.Tool{
		Name:        "spending_by_category",
		Description: "Total expense per top-level category over a date range, in base currency.",
	}, s.spendingByCategory)

	mcpwrap.AddTool(srv, &mcpwrap.Tool{
		Name:        "cash_flow",
		Description: "Income vs expense per calendar month over a date range, in base currency.",
	}, s.cashFlow)

	mcpwrap.AddTool(srv, &mcpwrap.Tool{
		Name:        "list_transactions",
		Description: "Search transactions with optional filters (account, category, type, date range, text query, uncategorized-only).",
	}, s.listTransactions)

	mcpwrap.AddTool(srv, &mcpwrap.Tool{
		Name:        "list_budgets",
		Description: "List configured budgets (per-category spending limits).",
	}, s.listBudgets)

	mcpwrap.AddTool(srv, &mcpwrap.Tool{
		Name:        "forecast",
		Description: "Projected income, expense and net for a month from scheduled transactions, budgets and credit-card usage, in base currency.",
	}, s.forecast)

	mcpwrap.AddTool(srv, &mcpwrap.Tool{
		Name:        "save_audit_report",
		Description: "Persist a finance-audit report (title, period, markdown summary, structured findings). The only tool that writes; it never touches the ledger.",
	}, s.saveAuditReport)

	mcpwrap.AddTool(srv, &mcpwrap.Tool{
		Name:        "list_audit_reports",
		Description: "List previously saved audit reports, most recent first.",
	}, s.listAuditReports)
}

type empty struct{}

// --- net_worth ---

type currencyOut struct {
	Currency    string       `json:"currency"`
	Assets      money.Money  `json:"assets"`
	Liabilities money.Money  `json:"liabilities"`
	Net         money.Money  `json:"net"`
	NetInBase   *money.Money `json:"net_in_base,omitempty"`
	RateKnown   bool         `json:"rate_known"`
}

type netWorthOut struct {
	Base         string        `json:"base"`
	Assets       money.Money   `json:"assets"`
	Liabilities  money.Money   `json:"liabilities"`
	Net          money.Money   `json:"net"`
	ByCurrency   []currencyOut `json:"by_currency"`
	MissingRates []string      `json:"missing_rates"`
}

func (s *Service) netWorth(ctx context.Context, _ *mcpwrap.CallToolRequest, _ empty) (*mcpwrap.CallToolResult, any, error) {
	accounts, err := s.accounts.List(ctx, false)
	if err != nil {
		return nil, nil, err
	}
	balances, err := s.accounts.Balances(ctx)
	if err != nil {
		return nil, nil, err
	}
	rates, err := s.reports.LatestRates(ctx)
	if err != nil {
		return nil, nil, err
	}

	abs := make([]ledger.AccountBalance, 0, len(accounts))
	for _, a := range accounts {
		if a.ExcludedFromNetWorth {
			continue
		}
		abs = append(abs, ledger.AccountBalance{Account: a, Balance: balances[a.ID]})
	}

	nw := ledger.ComputeNetWorth(s.base, abs, rates)
	out := netWorthOut{
		Base:         nw.Base,
		Assets:       nw.Assets,
		Liabilities:  nw.Liabilities,
		Net:          nw.Net,
		MissingRates: nw.MissingRates,
	}
	if out.MissingRates == nil {
		out.MissingRates = []string{}
	}
	for _, e := range nw.ByCurrency {
		ce := currencyOut{
			Currency:    e.Currency,
			Assets:      e.Assets,
			Liabilities: e.Liabilities,
			Net:         e.Net,
			RateKnown:   e.RateKnown,
		}
		if e.RateKnown {
			nib := e.NetInBase
			ce.NetInBase = &nib
		}
		out.ByCurrency = append(out.ByCurrency, ce)
	}

	return result(out)
}

// --- list_accounts ---

type accountOut struct {
	ID                   uuid.UUID   `json:"id"`
	Name                 string      `json:"name"`
	Kind                 string      `json:"kind"`
	Type                 string      `json:"type"`
	Currency             string      `json:"currency"`
	Balance              money.Money `json:"balance"`
	Archived             bool        `json:"archived"`
	ExcludedFromNetWorth bool        `json:"excluded_from_net_worth"`
}

type listAccountsIn struct {
	IncludeArchived bool `json:"include_archived,omitempty" jsonschema:"include archived accounts (default false)"`
}

type listAccountsOut struct {
	Accounts []accountOut `json:"accounts"`
}

func (s *Service) listAccounts(ctx context.Context, _ *mcpwrap.CallToolRequest, in listAccountsIn) (*mcpwrap.CallToolResult, any, error) {
	accounts, err := s.accounts.List(ctx, in.IncludeArchived)
	if err != nil {
		return nil, nil, err
	}
	balances, err := s.accounts.Balances(ctx)
	if err != nil {
		return nil, nil, err
	}

	out := listAccountsOut{Accounts: make([]accountOut, len(accounts))}
	for i, a := range accounts {
		out.Accounts[i] = accountOut{
			ID:                   a.ID,
			Name:                 a.Name,
			Kind:                 string(a.Kind),
			Type:                 string(a.Type),
			Currency:             a.Currency,
			Balance:              balances[a.ID],
			Archived:             a.Archived,
			ExcludedFromNetWorth: a.ExcludedFromNetWorth,
		}
	}

	return result(out)
}

// --- list_categories ---

type categoryOut struct {
	ID       uuid.UUID  `json:"id"`
	Name     string     `json:"name"`
	Type     string     `json:"type"`
	ParentID *uuid.UUID `json:"parent_id,omitempty"`
	Archived bool       `json:"archived"`
}

type listCategoriesIn struct {
	Type            string `json:"type,omitempty" jsonschema:"filter by 'expense' or 'income'; empty for both"`
	IncludeArchived bool   `json:"include_archived,omitempty" jsonschema:"include archived categories (default false)"`
}

type listCategoriesOut struct {
	Categories []categoryOut `json:"categories"`
}

func (s *Service) listCategories(ctx context.Context, _ *mcpwrap.CallToolRequest, in listCategoriesIn) (*mcpwrap.CallToolResult, any, error) {
	var typ *entities.CategoryType
	if in.Type != "" {
		t := entities.CategoryType(in.Type)
		typ = &t
	}

	cats, err := s.categories.List(ctx, typ, in.IncludeArchived)
	if err != nil {
		return nil, nil, err
	}

	out := listCategoriesOut{Categories: make([]categoryOut, len(cats))}
	for i, c := range cats {
		out.Categories[i] = categoryOut{
			ID:       c.ID,
			Name:     c.Name,
			Type:     string(c.Type),
			ParentID: c.ParentID,
			Archived: c.Archived,
		}
	}

	return result(out)
}

// --- spending_by_category ---

type dateRangeIn struct {
	DateFrom string `json:"date_from,omitempty" jsonschema:"start date YYYY-MM-DD (inclusive); empty for all time"`
	DateTo   string `json:"date_to,omitempty" jsonschema:"end date YYYY-MM-DD (inclusive); empty for now"`
}

type categorySpendOut struct {
	CategoryID   uuid.UUID   `json:"category_id"`
	CategoryName string      `json:"category_name"`
	Amount       money.Money `json:"amount"`
}

type spendingOut struct {
	Base       string             `json:"base"`
	Total      money.Money        `json:"total"`
	Categories []categorySpendOut `json:"categories"`
}

func (s *Service) spendingByCategory(ctx context.Context, _ *mcpwrap.CallToolRequest, in dateRangeIn) (*mcpwrap.CallToolResult, any, error) {
	from, to, err := dateWindow(in.DateFrom, in.DateTo)
	if err != nil {
		return nil, nil, err
	}

	spends, err := s.reports.SpendingByCategory(ctx, from, to)
	if err != nil {
		return nil, nil, err
	}

	total := money.Zero(s.base)
	out := spendingOut{Base: s.base, Categories: make([]categorySpendOut, len(spends))}
	for i, sp := range spends {
		if total, err = total.Plus(sp.Amount); err != nil {
			return nil, nil, err
		}
		out.Categories[i] = categorySpendOut{
			CategoryID:   sp.CategoryID,
			CategoryName: sp.CategoryName,
			Amount:       sp.Amount,
		}
	}
	out.Total = total

	return result(out)
}

// --- cash_flow ---

type monthFlowOut struct {
	Month   string      `json:"month"`
	Income  money.Money `json:"income"`
	Expense money.Money `json:"expense"`
	Net     money.Money `json:"net"`
}

type cashFlowOut struct {
	Base   string         `json:"base"`
	Months []monthFlowOut `json:"months"`
}

func (s *Service) cashFlow(ctx context.Context, _ *mcpwrap.CallToolRequest, in dateRangeIn) (*mcpwrap.CallToolResult, any, error) {
	from, to, err := dateWindow(in.DateFrom, in.DateTo)
	if err != nil {
		return nil, nil, err
	}

	flows, err := s.reports.CashFlow(ctx, from, to)
	if err != nil {
		return nil, nil, err
	}

	out := cashFlowOut{Base: s.base, Months: make([]monthFlowOut, len(flows))}
	for i, f := range flows {
		net, nerr := f.Income.Minus(f.Expense)
		if nerr != nil {
			return nil, nil, nerr
		}
		out.Months[i] = monthFlowOut{Month: f.Month, Income: f.Income, Expense: f.Expense, Net: net}
	}

	return result(out)
}

// --- list_transactions ---

type listTransactionsIn struct {
	AccountID     string `json:"account_id,omitempty" jsonschema:"filter by account uuid (from or to)"`
	CategoryID    string `json:"category_id,omitempty" jsonschema:"filter by category uuid"`
	Type          string `json:"type,omitempty" jsonschema:"filter by 'expense', 'income' or 'transfer'"`
	DateFrom      string `json:"date_from,omitempty" jsonschema:"start date YYYY-MM-DD inclusive"`
	DateTo        string `json:"date_to,omitempty" jsonschema:"end date YYYY-MM-DD inclusive"`
	Query         string `json:"query,omitempty" jsonschema:"free-text search over note/tags"`
	Uncategorized bool   `json:"uncategorized,omitempty" jsonschema:"only transactions with no category"`
	Limit         int    `json:"limit,omitempty" jsonschema:"max rows (default 100)"`
	Offset        int    `json:"offset,omitempty" jsonschema:"rows to skip for paging"`
}

type transactionOut struct {
	ID            uuid.UUID    `json:"id"`
	Date          string       `json:"date"`
	Type          string       `json:"type"`
	FromAccountID *uuid.UUID   `json:"from_account_id,omitempty"`
	ToAccountID   *uuid.UUID   `json:"to_account_id,omitempty"`
	CategoryID    *uuid.UUID   `json:"category_id,omitempty"`
	Amount        money.Money  `json:"amount"`
	BaseAmount    *money.Money `json:"base_amount,omitempty"`
	Note          *string      `json:"note,omitempty"`
	Tags          []string     `json:"tags,omitempty"`
}

type listTransactionsOut struct {
	Transactions []transactionOut `json:"transactions"`
}

func (s *Service) listTransactions(ctx context.Context, _ *mcpwrap.CallToolRequest, in listTransactionsIn) (*mcpwrap.CallToolResult, any, error) {
	filter := transactionrepository.Filter{
		Uncategorized: in.Uncategorized,
		Limit:         in.Limit,
		Offset:        in.Offset,
	}
	if filter.Limit <= 0 {
		filter.Limit = 100
	}

	if in.AccountID != "" {
		id, err := uuid.Parse(in.AccountID)
		if err != nil {
			return nil, nil, fmt.Errorf("account_id: %w", err)
		}
		filter.AccountID = &id
	}
	if in.CategoryID != "" {
		id, err := uuid.Parse(in.CategoryID)
		if err != nil {
			return nil, nil, fmt.Errorf("category_id: %w", err)
		}
		filter.CategoryID = &id
	}
	if in.Type != "" {
		t := entities.TransactionType(in.Type)
		filter.Type = &t
	}
	if in.Query != "" {
		q := in.Query
		filter.Query = &q
	}
	if t, ok, err := parseDate(in.DateFrom); err != nil {
		return nil, nil, fmt.Errorf("date_from: %w", err)
	} else if ok {
		filter.DateFrom = &t
	}
	if t, ok, err := parseDate(in.DateTo); err != nil {
		return nil, nil, fmt.Errorf("date_to: %w", err)
	} else if ok {
		filter.DateTo = &t
	}

	txs, err := s.transactions.List(ctx, filter)
	if err != nil {
		return nil, nil, err
	}

	out := listTransactionsOut{Transactions: make([]transactionOut, len(txs))}
	for i, t := range txs {
		out.Transactions[i] = transactionOut{
			ID:            t.ID,
			Date:          t.Date.Format("2006-01-02"),
			Type:          string(t.Type),
			FromAccountID: t.FromAccountID,
			ToAccountID:   t.ToAccountID,
			CategoryID:    t.CategoryID,
			Amount:        t.Amount,
			BaseAmount:    t.BaseAmount,
			Note:          t.Note,
			Tags:          t.Tags,
		}
	}

	return result(out)
}

// --- list_budgets ---

type budgetOut struct {
	ID         uuid.UUID   `json:"id"`
	CategoryID uuid.UUID   `json:"category_id"`
	Period     string      `json:"period"`
	Amount     money.Money `json:"amount"`
	Rollover   bool        `json:"rollover"`
}

type listBudgetsOut struct {
	Budgets []budgetOut `json:"budgets"`
}

func (s *Service) listBudgets(ctx context.Context, _ *mcpwrap.CallToolRequest, _ empty) (*mcpwrap.CallToolResult, any, error) {
	budgets, err := s.budgets.List(ctx)
	if err != nil {
		return nil, nil, err
	}

	out := listBudgetsOut{Budgets: make([]budgetOut, len(budgets))}
	for i, b := range budgets {
		out.Budgets[i] = budgetOut{
			ID:         b.ID,
			CategoryID: b.CategoryID,
			Period:     string(b.Period),
			Amount:     b.Amount,
			Rollover:   b.Rollover,
		}
	}

	return result(out)
}
