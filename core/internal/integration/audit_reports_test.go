package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"finance/internal/entities"
	auditreportrepository "finance/internal/repositories/audit_report_repository"
)

func auditReportRepo() auditreportrepository.Repository {
	return auditreportrepository.NewRepository(testDB)
}

func TestAuditReportRepository_CreateAndList(t *testing.T) {
	ctx := context.Background()
	_, err := testDB.Pool.Exec(ctx, `TRUNCATE audit_reports RESTART IDENTITY`)
	require.NoError(t, err)

	repo := auditReportRepo()
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)

	report := entities.AuditReport{
		Title:      "Q1 audit",
		PeriodFrom: &from,
		PeriodTo:   &to,
		Summary:    "## Findings\nLooks healthy.",
		Findings: []entities.Finding{
			{Severity: "warning", Category: "spending", Message: "Dining up 30%"},
			{Severity: "info", Category: "data", Message: "2 uncategorized transactions"},
		},
	}

	require.NoError(t, repo.Create(ctx, &report))
	require.NotEqual(t, [16]byte{}, [16]byte(report.ID), "id back-filled")
	require.False(t, report.CreatedAt.IsZero(), "created_at back-filled")

	// A second report with no period / no findings round-trips too.
	bare := entities.AuditReport{Title: "bare"}
	require.NoError(t, repo.Create(ctx, &bare))

	got, err := repo.List(ctx, 10)
	require.NoError(t, err)
	require.Len(t, got, 2)

	// Most recent first.
	require.Equal(t, "bare", got[0].Title)
	require.Empty(t, got[0].Findings)

	q1 := got[1]
	require.Equal(t, "Q1 audit", q1.Title)
	require.Equal(t, "## Findings\nLooks healthy.", q1.Summary)
	require.NotNil(t, q1.PeriodFrom)
	require.Equal(t, from, q1.PeriodFrom.UTC())
	require.NotNil(t, q1.PeriodTo)
	require.Len(t, q1.Findings, 2)
	require.Equal(t, "warning", q1.Findings[0].Severity)
	require.Equal(t, "Dining up 30%", q1.Findings[0].Message)
}
