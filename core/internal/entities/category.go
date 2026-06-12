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
	ID        uuid.UUID
	Name      string
	ParentID  *uuid.UUID
	Type      CategoryType
	Icon      *string
	Color     *string
	Archived  bool
	CreatedAt time.Time
}
