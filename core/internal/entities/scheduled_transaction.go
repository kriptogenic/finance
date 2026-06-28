package entities

import (
	"time"

	"github.com/google/uuid"

	"finance/pkg/fx"
	"finance/pkg/money"
)

// ScheduleFrequency is the unit a scheduled transaction recurs in; combined with
// an interval it yields the cadence (e.g. monthly × 1, weekly × 2).
type ScheduleFrequency string

const (
	FreqDaily   ScheduleFrequency = "daily"
	FreqWeekly  ScheduleFrequency = "weekly"
	FreqMonthly ScheduleFrequency = "monthly"
	FreqYearly  ScheduleFrequency = "yearly"
)

// ScheduledTransaction is a transaction template plus a recurrence rule. The
// template mirrors Transaction's buckets (§4/§5); a worker materializes it into
// real transactions, advancing NextRun each run.
type ScheduledTransaction struct {
	ID   uuid.UUID `db:"id"`
	Name *string   `db:"name"`

	Type          TransactionType `db:"type"`
	FromAccountID *uuid.UUID      `db:"from_account_id"`
	ToAccountID   *uuid.UUID      `db:"to_account_id"`
	CategoryID    *uuid.UUID      `db:"category_id"`
	Amount        money.Money     `db:"amount"`
	ToAmount      *money.Money    `db:"to_amount"`
	// Reads select rate_to_base::text AS rate_to_base so fx.Rate scans from the decimal.
	RateToBase *fx.Rate `db:"rate_to_base"`
	Note       *string  `db:"note"`
	Tags       []string `db:"tags"`

	Frequency ScheduleFrequency `db:"frequency"`
	Interval  int               `db:"interval"`
	NextRun   time.Time         `db:"next_run"` // date of the next occurrence to materialize
	EndDate   *time.Time        `db:"end_date"`
	Paused    bool              `db:"paused"`
	LastRunAt *time.Time        `db:"last_run_at"`
	CreatedAt time.Time         `db:"created_at"`
}

// NextRun returns the occurrence date that follows from, stepping by interval
// units of freq. Calendar-aligned via AddDate, so month/year steps roll over
// naturally (e.g. Jan 31 + 1 month → Mar 3 in a non-leap year, matching Go's
// time semantics). interval is clamped to at least 1.
func NextRun(freq ScheduleFrequency, interval int, from time.Time) time.Time {
	if interval < 1 {
		interval = 1
	}

	switch freq {
	case FreqDaily:
		return from.AddDate(0, 0, interval)
	case FreqWeekly:
		return from.AddDate(0, 0, 7*interval)
	case FreqYearly:
		return from.AddDate(interval, 0, 0)
	default: // monthly
		return from.AddDate(0, interval, 0)
	}
}
