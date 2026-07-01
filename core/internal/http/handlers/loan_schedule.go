package handlers

import (
	"context"
	"errors"
	"time"

	"github.com/oapi-codegen/nullable"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"finance/generated/api"
	"finance/internal/entities"
	"finance/internal/ledger"
	accountrepository "finance/internal/repositories/account_repository"
	"finance/pkg/money"
)

// GetLoanSchedule returns the stored repayment schedule, generating and
// persisting it from the loan's terms on first read.
func (s Server) GetLoanSchedule(ctx context.Context, request api.GetLoanScheduleRequestObject) (api.GetLoanScheduleResponseObject, error) {
	acc, errMsg, err := s.loanForSchedule(ctx, request.Id)
	switch {
	case errors.Is(err, accountrepository.ErrNotFound):
		return api.GetLoanSchedule404JSONResponse{NotFoundJSONResponse: notFound("account not found")}, nil
	case err != nil:
		return nil, err
	case errMsg != "":
		return api.GetLoanSchedule400JSONResponse{BadRequestJSONResponse: badRequest(errMsg)}, nil
	}

	rows, err := s.loanSchedules.ListByAccount(ctx, acc.ID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		if rows, err = s.regenerate(ctx, acc, nil, nil); err != nil {
			return nil, err
		}
	}

	return api.GetLoanSchedule200JSONResponse(toLoanSchedule(acc.Currency, rows)), nil
}

// RegenerateLoanSchedule rebuilds the schedule from the loan's terms while
// preserving any manual date overrides and paid flags already recorded.
func (s Server) RegenerateLoanSchedule(ctx context.Context, request api.RegenerateLoanScheduleRequestObject) (api.RegenerateLoanScheduleResponseObject, error) {
	acc, errMsg, err := s.loanForSchedule(ctx, request.Id)
	switch {
	case errors.Is(err, accountrepository.ErrNotFound):
		return api.RegenerateLoanSchedule404JSONResponse{NotFoundJSONResponse: notFound("account not found")}, nil
	case err != nil:
		return nil, err
	case errMsg != "":
		return api.RegenerateLoanSchedule400JSONResponse{BadRequestJSONResponse: badRequest(errMsg)}, nil
	}

	existing, err := s.loanSchedules.ListByAccount(ctx, acc.ID)
	if err != nil {
		return nil, err
	}
	overrides, paid := overridesAndPaid(existing)

	rows, err := s.regenerate(ctx, acc, overrides, paid)
	if err != nil {
		return nil, err
	}

	return api.RegenerateLoanSchedule200JSONResponse(toLoanSchedule(acc.Currency, rows)), nil
}

// UpdateLoanScheduleRow overrides a due date or marks an installment paid, then
// recomputes the whole schedule forward from the loan's terms.
func (s Server) UpdateLoanScheduleRow(ctx context.Context, request api.UpdateLoanScheduleRowRequestObject) (api.UpdateLoanScheduleRowResponseObject, error) {
	acc, errMsg, err := s.loanForSchedule(ctx, request.Id)
	switch {
	case errors.Is(err, accountrepository.ErrNotFound):
		return api.UpdateLoanScheduleRow404JSONResponse{NotFoundJSONResponse: notFound("account not found")}, nil
	case err != nil:
		return nil, err
	case errMsg != "":
		return api.UpdateLoanScheduleRow400JSONResponse{BadRequestJSONResponse: badRequest(errMsg)}, nil
	}
	if request.Period < 1 || request.Period > *acc.TermMonths {
		return api.UpdateLoanScheduleRow400JSONResponse{BadRequestJSONResponse: badRequest("period out of range")}, nil
	}

	existing, err := s.loanSchedules.ListByAccount(ctx, acc.ID)
	if err != nil {
		return nil, err
	}
	overrides, paid := overridesAndPaid(existing)

	if body := request.Body; body != nil {
		if body.DateOverride.IsSpecified() {
			if body.DateOverride.IsNull() {
				delete(overrides, request.Period)
			} else {
				d, dErr := body.DateOverride.Get()
				if dErr != nil {
					return api.UpdateLoanScheduleRow400JSONResponse{BadRequestJSONResponse: badRequest("invalid date_override")}, nil
				}
				overrides[request.Period] = d.Time.UTC()
			}
		}
		if body.Paid != nil {
			paid[request.Period] = *body.Paid
		}
	}

	rows, err := s.regenerate(ctx, acc, overrides, paid)
	if err != nil {
		return nil, err
	}

	return api.UpdateLoanScheduleRow200JSONResponse(toLoanSchedule(acc.Currency, rows)), nil
}

// loanForSchedule loads the account and checks it is a loan with complete terms.
// A non-empty message is a 400 reason; ErrNotFound signals a 404.
func (s Server) loanForSchedule(ctx context.Context, id openapi_types.UUID) (*entities.Account, string, error) {
	acc, err := s.accounts.Get(ctx, id)
	if err != nil {
		return nil, "", err
	}
	if acc.Type != entities.TypeLoan {
		return acc, "account is not a loan", nil
	}
	if acc.Principal == nil || acc.InterestRate == nil || acc.TermMonths == nil || acc.StartDate == nil {
		return acc, "loan terms are incomplete (need principal, interest_rate, term_months, start_date)", nil
	}

	return acc, "", nil
}

// regenerate builds the schedule from the loan's terms + the holiday calendar,
// applying the given overrides/paid flags, persists it, and returns the stored
// rows (with ids).
func (s Server) regenerate(ctx context.Context, acc *entities.Account, overrides map[int]time.Time, paid map[int]bool) ([]entities.LoanSchedule, error) {
	holidays, err := s.holidays.List(ctx)
	if err != nil {
		return nil, err
	}
	days := make([]time.Time, len(holidays))
	for i, h := range holidays {
		days[i] = h.Day
	}

	paymentDay := 0
	if acc.PaymentDay != nil {
		paymentDay = *acc.PaymentDay
	}

	sched, err := ledger.GenerateSchedule(ledger.ScheduleParams{
		Principal:  *acc.Principal,
		AnnualRate: *acc.InterestRate,
		TermMonths: *acc.TermMonths,
		Start:      *acc.StartDate,
		PaymentDay: paymentDay,
		Calendar:   ledger.NewHolidaySet(days),
		Overrides:  overrides,
	})
	if err != nil {
		return nil, err
	}

	rows := make([]entities.LoanSchedule, len(sched.Rows))
	for i, r := range sched.Rows {
		row := entities.LoanSchedule{
			AccountID: acc.ID,
			Period:    r.Period,
			DueDate:   r.Date,
			Payment:   r.Payment,
			Principal: r.Principal,
			Interest:  r.Interest,
			Balance:   r.Balance,
			Paid:      paid[r.Period],
		}
		if ov, ok := overrides[r.Period]; ok {
			row.DateOverride = &ov
		}
		rows[i] = row
	}

	if err = s.loanSchedules.Replace(ctx, acc.ID, rows); err != nil {
		return nil, err
	}

	return s.loanSchedules.ListByAccount(ctx, acc.ID)
}

// overridesAndPaid extracts the manual overrides and paid flags from stored rows
// so a regenerate preserves them.
func overridesAndPaid(rows []entities.LoanSchedule) (map[int]time.Time, map[int]bool) {
	overrides := make(map[int]time.Time)
	paid := make(map[int]bool)
	for _, r := range rows {
		if r.DateOverride != nil {
			overrides[r.Period] = *r.DateOverride
		}
		if r.Paid {
			paid[r.Period] = true
		}
	}

	return overrides, paid
}

func toLoanSchedule(currency string, rows []entities.LoanSchedule) api.LoanScheduleResponse {
	out := make([]api.LoanScheduleRow, len(rows))
	var totalPayment, totalInterest int64
	for i, r := range rows {
		row := api.LoanScheduleRow{
			Id:        r.ID,
			Period:    r.Period,
			DueDate:   openapi_types.Date{Time: r.DueDate},
			Payment:   r.Payment,
			Principal: r.Principal,
			Interest:  r.Interest,
			Balance:   r.Balance,
			Paid:      r.Paid,
		}
		if r.DateOverride != nil {
			row.DateOverride = nullable.NewNullableWithValue(openapi_types.Date{Time: *r.DateOverride})
		}
		out[i] = row
		totalPayment += r.Payment.Minor()
		totalInterest += r.Interest.Minor()
	}

	resp := api.LoanScheduleResponse{
		Currency:      currency,
		TotalPayment:  money.New(totalPayment, currency),
		TotalInterest: money.New(totalInterest, currency),
		Rows:          out,
	}
	if len(rows) > 0 {
		resp.MonthlyPayment = rows[0].Payment
	} else {
		resp.MonthlyPayment = money.New(0, currency)
	}

	return resp
}
