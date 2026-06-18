package entities

import (
	"time"

	"github.com/google/uuid"
)

type CategoryRule struct {
	ID         uuid.UUID
	Pattern    string
	CategoryID uuid.UUID
	CreatedAt  time.Time
}
