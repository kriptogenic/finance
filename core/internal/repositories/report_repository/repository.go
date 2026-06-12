package reportrepository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"finance/pkg/database"
	"finance/pkg/fx"
)

// CategorySpend is total expense in a top-level category over a period, in base
// currency minor units.
type CategorySpend struct {
	CategoryID   uuid.UUID
	CategoryName string
	Amount       int64
}

// MonthFlow is income vs expense for one calendar month, in base minor units.
type MonthFlow struct {
	Month   string // YYYY-MM
	Income  int64
	Expense int64
}

type Repository interface {
	// LatestRates returns the most recent frozen rate_to_base per currency — the
	// "latest known rate" used to convert current balances (§3 point 5).
	LatestRates(ctx context.Context) (map[string]fx.Rate, error)
	SpendingByCategory(ctx context.Context, from, to time.Time) ([]CategorySpend, error)
	CashFlow(ctx context.Context, from, to time.Time) ([]MonthFlow, error)
}

type repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) Repository {
	return &repository{db: db}
}

func (r repository) LatestRates(ctx context.Context) (map[string]fx.Rate, error) {
	const query = `
		SELECT DISTINCT ON (currency) currency, rate_to_base::text
		FROM transactions
		WHERE rate_to_base IS NOT NULL
		ORDER BY currency, date DESC, created_at DESC`

	rows, err := r.db.Pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("latest rates: %w", err)
	}
	defer rows.Close()

	rates := make(map[string]fx.Rate)
	for rows.Next() {
		var currency, rateText string
		if err = rows.Scan(&currency, &rateText); err != nil {
			return nil, fmt.Errorf("latest rates: %w", err)
		}

		rate, parseErr := fx.ParseRate(rateText)
		if parseErr != nil {
			return nil, fmt.Errorf("latest rates: %w", parseErr)
		}

		rates[currency] = rate
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("latest rates: %w", err)
	}

	return rates, nil
}

func (r repository) SpendingByCategory(ctx context.Context, from, to time.Time) ([]CategorySpend, error) {
	// base_amount is the frozen base value; it is NULL only when the currency is
	// already base, in which case amount is the base value — hence COALESCE.
	// Subcategory spend rolls up to its top-level parent.
	const query = `
		SELECT top.id, top.name, SUM(COALESCE(t.base_amount, t.amount))::bigint AS spent
		FROM transactions t
		JOIN categories child ON child.id = t.category_id
		JOIN categories top   ON top.id = COALESCE(child.parent_id, child.id)
		WHERE t.type = 'expense' AND t.date >= $1 AND t.date <= $2
		GROUP BY top.id, top.name
		ORDER BY spent DESC`

	rows, err := r.db.Pool.Query(ctx, query, from, to)
	if err != nil {
		return nil, fmt.Errorf("spending by category: %w", err)
	}
	defer rows.Close()

	var out []CategorySpend
	for rows.Next() {
		var c CategorySpend
		if err = rows.Scan(&c.CategoryID, &c.CategoryName, &c.Amount); err != nil {
			return nil, fmt.Errorf("spending by category: %w", err)
		}

		out = append(out, c)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("spending by category: %w", err)
	}

	return out, nil
}

func (r repository) CashFlow(ctx context.Context, from, to time.Time) ([]MonthFlow, error) {
	const query = `
		SELECT to_char(date_trunc('month', date), 'YYYY-MM') AS month,
		       SUM(CASE WHEN type = 'income'  THEN COALESCE(base_amount, amount) ELSE 0 END)::bigint AS income,
		       SUM(CASE WHEN type = 'expense' THEN COALESCE(base_amount, amount) ELSE 0 END)::bigint AS expense
		FROM transactions
		WHERE type IN ('income', 'expense') AND date >= $1 AND date <= $2
		GROUP BY 1
		ORDER BY 1`

	rows, err := r.db.Pool.Query(ctx, query, from, to)
	if err != nil {
		return nil, fmt.Errorf("cash flow: %w", err)
	}
	defer rows.Close()

	var out []MonthFlow
	for rows.Next() {
		var m MonthFlow
		if err = rows.Scan(&m.Month, &m.Income, &m.Expense); err != nil {
			return nil, fmt.Errorf("cash flow: %w", err)
		}

		out = append(out, m)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("cash flow: %w", err)
	}

	return out, nil
}
