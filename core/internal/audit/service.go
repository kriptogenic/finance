// Package audit exposes read-only finance data plus a single audit-report write
// over MCP, so Claude can audit the ledger without ever mutating it.
package audit

import (
	"finance/config"
	accountrepository "finance/internal/repositories/account_repository"
	auditreportrepository "finance/internal/repositories/audit_report_repository"
	budgetrepository "finance/internal/repositories/budget_repository"
	categoryrepository "finance/internal/repositories/category_repository"
	reportrepository "finance/internal/repositories/report_repository"
	scheduledtransactionrepository "finance/internal/repositories/scheduled_transaction_repository"
	transactionrepository "finance/internal/repositories/transaction_repository"
)

// Service holds the read repositories the audit tools query plus the
// audit_reports writer. Every tool but save_audit_report is read-only.
type Service struct {
	accounts     accountrepository.Repository
	reports      reportrepository.Repository
	transactions transactionrepository.Repository
	budgets      budgetrepository.Repository
	categories   categoryrepository.Repository
	schedules    scheduledtransactionrepository.Repository
	audit        auditreportrepository.Repository
	base         string
}

func New(
	accounts accountrepository.Repository,
	reports reportrepository.Repository,
	transactions transactionrepository.Repository,
	budgets budgetrepository.Repository,
	categories categoryrepository.Repository,
	schedules scheduledtransactionrepository.Repository,
	auditReports auditreportrepository.Repository,
	finance *config.Finance,
) *Service {
	return &Service{
		accounts:     accounts,
		reports:      reports,
		transactions: transactions,
		budgets:      budgets,
		categories:   categories,
		schedules:    schedules,
		audit:        auditReports,
		base:         finance.BaseCurrency,
	}
}
