package handlers

import (
	"bytes"
	"context"
	"math"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"finance/generated/api"
	"finance/internal/entities"
	transactionrepository "finance/internal/repositories/transaction_repository"
	"finance/pkg/xlsx"
)

// ExportTransactions streams matching expenses and income as an .xlsx file with
// datetime, amount, category name and note columns. Transfers are excluded;
// expenses are written as negative amounts so the column reads as a signed flow.
func (s Server) ExportTransactions(ctx context.Context, request api.ExportTransactionsRequestObject) (api.ExportTransactionsResponseObject, error) {
	p := request.Params
	filter := transactionrepository.Filter{
		AccountID:  p.AccountId,
		CategoryID: p.CategoryId,
		DateFrom:   p.DateFrom,
		DateTo:     p.DateTo,
		Tag:        p.Tag,
		Query:      p.Q,
		Limit:      math.MaxInt32, // export every matching row, not a page
	}
	if p.Type != nil {
		t := entities.TransactionType(*p.Type)
		filter.Type = &t
	}

	txns, err := s.transactions.List(ctx, filter)
	if err != nil {
		return nil, err
	}

	// category id → name; include archived so historical rows still resolve
	cats, err := s.categories.List(ctx, nil, true)
	if err != nil {
		return nil, err
	}
	catName := make(map[uuid.UUID]string, len(cats))
	for _, c := range cats {
		catName[c.ID] = c.Name
	}

	wb := xlsx.New(xlsx.WithSheet("Transactions"))
	defer func() {
		if cerr := wb.Close(); cerr != nil {
			s.logger.Warn("close export workbook", zap.Error(cerr))
		}
	}()

	if err = wb.Header("Date", "Amount", "Currency", "Category", "Note"); err != nil {
		return nil, err
	}
	for _, t := range txns {
		if t.Type == entities.TxTransfer {
			continue // transfers carry no category and aren't income/expense
		}

		amount := t.Amount.AsMajorUnits()
		if t.Type == entities.TxExpense {
			amount = -amount
		}

		category := ""
		if t.CategoryID != nil {
			category = catName[*t.CategoryID]
		}
		note := ""
		if t.Note != nil {
			note = *t.Note
		}

		if err = wb.AppendRow(t.Date.Format("2006-01-02 15:04:05"), amount, t.Amount.Code(), category, note); err != nil {
			return nil, err
		}
	}

	// widths (chars) per column, then a thousands/red-negative format on Amount,
	// and a filter/sort dropdown on the header.
	for col, width := range []float64{20, 14, 10, 24, 44} {
		if err = wb.SetColWidth(col+1, width); err != nil {
			return nil, err
		}
	}
	if err = wb.SetColNumberFormat(2, "#,##0.00;[Red]-#,##0.00"); err != nil {
		return nil, err
	}
	if err = wb.AutoFilter(); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if _, err = wb.WriteTo(&buf); err != nil {
		return nil, err
	}

	disposition := `attachment; filename="transactions-` + time.Now().Format("2006-01-02") + `.xlsx"`

	return api.ExportTransactions200ApplicationvndOpenxmlformatsOfficedocumentSpreadsheetmlSheetResponse{
		Body:          &buf,
		ContentLength: int64(buf.Len()),
		Headers: api.ExportTransactions200ResponseHeaders{
			ContentDisposition: &disposition,
		},
	}, nil
}
