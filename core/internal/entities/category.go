package entities

import (
	"time"

	"github.com/google/uuid"
)

// CategoryType is the root of the two category trees. An expense/income
// transaction's category must match its own type (§5).
type CategoryType string

const (
	CategoryExpense CategoryType = "expense"
	CategoryIncome  CategoryType = "income"
)

// Category is a money bucket for income sources or expense destinations. The
// tree is two levels (category → subcategory); a subcategory shares its
// parent's type.
type Category struct {
	ID       uuid.UUID    `db:"id"`
	Name     string       `db:"name"`
	ParentID *uuid.UUID   `db:"parent_id"`
	Type     CategoryType `db:"type"`
	Icon     *string      `db:"icon"`
	Color    *string      `db:"color"`
	Archived bool         `db:"archived"`

	// HiddenInPicker hides the category from the categorize picker; it stays
	// usable by ingest rules and on existing transactions.
	HiddenInPicker bool `db:"hidden_in_picker"`

	CreatedAt time.Time `db:"created_at"`

	SystemKey *string `db:"system_key"` // marks built-in buckets (e.g. "uncategorized_expense") for ingest
}
