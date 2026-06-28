package entities

import (
	"time"

	"github.com/google/uuid"
)

// CategoryRule routes a merchant substring to a category. A nil CategoryID is a
// "block": a merchant the user never wants offered as a rule (it never routes).
type CategoryRule struct {
	ID         uuid.UUID  `db:"id"`
	Pattern    string     `db:"pattern"`
	CategoryID *uuid.UUID `db:"category_id"`
	CreatedAt  time.Time  `db:"created_at"`
}
