package entities

import (
	"time"

	"github.com/google/uuid"
)

// Finding is a single issue Claude surfaced during an audit, stored in the
// audit_reports.findings jsonb array.
type Finding struct {
	Severity string `json:"severity"` // info | warning | critical
	Category string `json:"category"` // free-form area, e.g. "spending", "data"
	Message  string `json:"message"`
}

// AuditReport is a persisted finance-audit result written by the MCP
// save_audit_report tool. The ledger itself is never mutated over MCP.
type AuditReport struct {
	ID         uuid.UUID  `db:"id"`
	Title      string     `db:"title"`
	PeriodFrom *time.Time `db:"period_from"`
	PeriodTo   *time.Time `db:"period_to"`
	Summary    string     `db:"summary"` // markdown body
	Findings   []Finding  `db:"findings"`
	CreatedAt  time.Time  `db:"created_at"`
}
