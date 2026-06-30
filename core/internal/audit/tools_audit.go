package audit

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"finance/internal/entities"
	mcpwrap "finance/pkg/mcp"
)

type findingIn struct {
	Severity string `json:"severity" jsonschema:"info | warning | critical"`
	Category string `json:"category" jsonschema:"area of the finding, e.g. spending, data, budget"`
	Message  string `json:"message" jsonschema:"the finding text"`
}

type saveAuditReportIn struct {
	Title      string      `json:"title" jsonschema:"short report title (required)"`
	PeriodFrom string      `json:"period_from,omitempty" jsonschema:"period start YYYY-MM-DD (optional)"`
	PeriodTo   string      `json:"period_to,omitempty" jsonschema:"period end YYYY-MM-DD (optional)"`
	Summary    string      `json:"summary,omitempty" jsonschema:"markdown summary of the audit"`
	Findings   []findingIn `json:"findings,omitempty" jsonschema:"structured findings"`
}

type saveAuditReportOut struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt string    `json:"created_at"`
}

func (s *Service) saveAuditReport(ctx context.Context, _ *mcpwrap.CallToolRequest, in saveAuditReportIn) (*mcpwrap.CallToolResult, any, error) {
	if in.Title == "" {
		return nil, nil, fmt.Errorf("title is required")
	}

	report := entities.AuditReport{
		Title:    in.Title,
		Summary:  in.Summary,
		Findings: make([]entities.Finding, len(in.Findings)),
	}
	for i, f := range in.Findings {
		report.Findings[i] = entities.Finding{Severity: f.Severity, Category: f.Category, Message: f.Message}
	}

	if t, ok, err := parseDate(in.PeriodFrom); err != nil {
		return nil, nil, fmt.Errorf("period_from: %w", err)
	} else if ok {
		report.PeriodFrom = &t
	}
	if t, ok, err := parseDate(in.PeriodTo); err != nil {
		return nil, nil, fmt.Errorf("period_to: %w", err)
	} else if ok {
		report.PeriodTo = &t
	}

	if err := s.audit.Create(ctx, &report); err != nil {
		return nil, nil, err
	}

	return result(saveAuditReportOut{ID: report.ID, CreatedAt: report.CreatedAt.Format("2006-01-02T15:04:05Z07:00")})
}

type auditReportOut struct {
	ID         uuid.UUID          `json:"id"`
	Title      string             `json:"title"`
	PeriodFrom *string            `json:"period_from,omitempty"`
	PeriodTo   *string            `json:"period_to,omitempty"`
	Summary    string             `json:"summary"`
	Findings   []entities.Finding `json:"findings"`
	CreatedAt  string             `json:"created_at"`
}

type listAuditReportsIn struct {
	Limit int `json:"limit,omitempty" jsonschema:"max reports to return (default 50)"`
}

type listAuditReportsOut struct {
	Reports []auditReportOut `json:"reports"`
}

func (s *Service) listAuditReports(ctx context.Context, _ *mcpwrap.CallToolRequest, in listAuditReportsIn) (*mcpwrap.CallToolResult, any, error) {
	reports, err := s.audit.List(ctx, in.Limit)
	if err != nil {
		return nil, nil, err
	}

	out := listAuditReportsOut{Reports: make([]auditReportOut, len(reports))}
	for i, r := range reports {
		ro := auditReportOut{
			ID:        r.ID,
			Title:     r.Title,
			Summary:   r.Summary,
			Findings:  r.Findings,
			CreatedAt: r.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
		if r.PeriodFrom != nil {
			d := r.PeriodFrom.Format("2006-01-02")
			ro.PeriodFrom = &d
		}
		if r.PeriodTo != nil {
			d := r.PeriodTo.Format("2006-01-02")
			ro.PeriodTo = &d
		}
		if ro.Findings == nil {
			ro.Findings = []entities.Finding{}
		}
		out.Reports[i] = ro
	}

	return result(out)
}
