package auditreportrepository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"

	"finance/internal/entities"
	"finance/pkg/database"
)

type Repository interface {
	// Create inserts a new audit report, back-filling id and created_at.
	Create(ctx context.Context, r *entities.AuditReport) error
	// List returns the most recent reports first, capped at limit.
	List(ctx context.Context, limit int) ([]entities.AuditReport, error)
}

type repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) Repository {
	return &repository{db: db}
}

const columns = `id, title, period_from, period_to, summary, findings, created_at`

func (r repository) Create(ctx context.Context, report *entities.AuditReport) error {
	findings, err := json.Marshal(report.Findings)
	if err != nil {
		return fmt.Errorf("create audit report: %w", err)
	}

	const query = `
		INSERT INTO audit_reports (title, period_from, period_to, summary, findings)
		VALUES ($1, $2, $3, $4, $5::jsonb)
		RETURNING id, created_at`

	err = r.db.Pool.QueryRow(ctx, query,
		report.Title, report.PeriodFrom, report.PeriodTo, report.Summary, string(findings)).
		Scan(&report.ID, &report.CreatedAt)
	if err != nil {
		return fmt.Errorf("create audit report: %w", err)
	}

	return nil
}

func (r repository) List(ctx context.Context, limit int) ([]entities.AuditReport, error) {
	if limit <= 0 {
		limit = 50
	}

	rows, err := r.db.Pool.Query(ctx,
		`SELECT `+columns+` FROM audit_reports ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list audit reports: %w", err)
	}

	reports, err := pgx.CollectRows(rows, pgx.RowToStructByName[entities.AuditReport])
	if err != nil {
		return nil, fmt.Errorf("list audit reports: %w", err)
	}

	return reports, nil
}
