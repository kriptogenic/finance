package entities

import (
	"time"

	"github.com/google/uuid"
)

// BudgetPeriod is the window a budget's limit applies to.
type BudgetPeriod string

const (
	BudgetWeekly  BudgetPeriod = "weekly"
	BudgetMonthly BudgetPeriod = "monthly"
	BudgetYearly  BudgetPeriod = "yearly"
)

// Budget is a spending limit for an expense category (and its subcategories)
// over a period. amount is in base-currency minor units (REQUIREMENTS App. A).
type Budget struct {
	ID          uuid.UUID
	CategoryID  uuid.UUID
	Period      BudgetPeriod
	Amount      int64
	Rollover    bool
	StartPeriod *time.Time
	CreatedAt   time.Time
}

// PeriodWindow returns the [start, end) window of the period containing ref,
// calendar-aligned (week starts Monday). Spend is summed over this window.
func PeriodWindow(period BudgetPeriod, ref time.Time) (start, end time.Time) {
	ref = ref.UTC()
	day := time.Date(ref.Year(), ref.Month(), ref.Day(), 0, 0, 0, 0, time.UTC)

	switch period {
	case BudgetWeekly:
		offset := (int(day.Weekday()) + 6) % 7 // days since Monday
		start = day.AddDate(0, 0, -offset)
		end = start.AddDate(0, 0, 7)
	case BudgetYearly:
		start = time.Date(ref.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
		end = start.AddDate(1, 0, 0)
	default: // monthly
		start = time.Date(ref.Year(), ref.Month(), 1, 0, 0, 0, 0, time.UTC)
		end = start.AddDate(0, 1, 0)
	}

	return start, end
}
