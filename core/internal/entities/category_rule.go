package entities

import (
	"time"

	"github.com/google/uuid"
)

// CategoryRule routes a merchant substring to a category. A nil CategoryID is a
// "block": a merchant the user never wants offered as a rule (it never routes).
type CategoryRule struct {
	ID         uuid.UUID
	Pattern    string
	CategoryID *uuid.UUID
	CreatedAt  time.Time
}
