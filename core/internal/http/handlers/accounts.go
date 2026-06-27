package handlers

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/oapi-codegen/nullable"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"go.uber.org/zap"

	"finance/generated/api"
	"finance/internal/entities"
	"finance/internal/ledger"
	accountrepository "finance/internal/repositories/account_repository"
	"finance/pkg/money"
)

func (s Server) ListAccounts(ctx context.Context, request api.ListAccountsRequestObject) (api.ListAccountsResponseObject, error) {
	includeArchived := request.Params.IncludeArchived != nil && *request.Params.IncludeArchived

	accounts, err := s.accounts.List(ctx, includeArchived)
	if err != nil {
		return nil, err
	}

	balances, err := s.accounts.Balances(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]api.Account, len(accounts))
	for i, acc := range accounts {
		out[i] = toAccount(acc, balances[acc.ID])
	}

	return api.ListAccounts200JSONResponse{Accounts: out}, nil
}

func (s Server) CreateAccount(ctx context.Context, request api.CreateAccountRequestObject) (api.CreateAccountResponseObject, error) {
	if request.Body == nil {
		return api.CreateAccount400JSONResponse{BadRequestJSONResponse: badRequest("empty body")}, nil
	}
	body := request.Body

	acc := entities.Account{
		Name:           body.Name,
		Kind:           entities.AccountKind(body.Kind),
		Type:           entities.AccountType(body.Type),
		Currency:       body.Currency,
		OpeningBalance: money.New(0, body.Currency),
		InterestRate:   floatPtr(body.InterestRate),
		TermMonths:     body.TermMonths,
		MaturityDate:   datePtr(body.MaturityDate),
		Capitalization: body.Capitalization,
		CreditLimit:    optMoney(body.CreditLimit, body.Currency),
		Principal:      optMoney(body.Principal, body.Currency),
		StartDate:      datePtr(body.StartDate),
		PaymentDay:     body.PaymentDay,
		CardLast4:      body.CardLast4,
	}
	if body.OpeningBalance != nil {
		acc.OpeningBalance = money.New(*body.OpeningBalance, body.Currency)
	}
	// default included; only an explicit false excludes it
	acc.ExcludedFromNetWorth = body.IncludeInNetWorth != nil && !*body.IncludeInNetWorth

	if !acc.ValidKindType() {
		return api.CreateAccount400JSONResponse{BadRequestJSONResponse: badRequest("kind does not match type")}, nil
	}

	if err := s.accounts.Create(ctx, &acc); err != nil {
		s.logger.Error("create account", zap.Error(err))

		return nil, err
	}

	// a fresh account has no transactions, so its balance is the opening balance
	return api.CreateAccount201JSONResponse(toAccount(acc, ledgerOpening(acc))), nil
}

func (s Server) GetAccount(ctx context.Context, request api.GetAccountRequestObject) (api.GetAccountResponseObject, error) {
	acc, err := s.accounts.Get(ctx, request.Id)
	if errors.Is(err, accountrepository.ErrNotFound) {
		return api.GetAccount404JSONResponse{NotFoundJSONResponse: notFound("account not found")}, nil
	}
	if err != nil {
		return nil, err
	}

	balance, err := s.accountBalance(ctx, acc.ID, *acc)
	if err != nil {
		return nil, err
	}

	return api.GetAccount200JSONResponse(toAccount(*acc, balance)), nil
}

func (s Server) UpdateAccount(ctx context.Context, request api.UpdateAccountRequestObject) (api.UpdateAccountResponseObject, error) {
	acc, err := s.accounts.Get(ctx, request.Id)
	if errors.Is(err, accountrepository.ErrNotFound) {
		return api.UpdateAccount404JSONResponse{NotFoundJSONResponse: notFound("account not found")}, nil
	}
	if err != nil {
		return nil, err
	}

	body := request.Body
	if body == nil {
		return api.UpdateAccount400JSONResponse{BadRequestJSONResponse: badRequest("empty body")}, nil
	}

	if body.Name != nil {
		if *body.Name == "" {
			return api.UpdateAccount400JSONResponse{BadRequestJSONResponse: badRequest("name must not be empty")}, nil
		}
		acc.Name = *body.Name
	}
	if body.Archived != nil {
		acc.Archived = *body.Archived
	}
	if body.IncludeInNetWorth != nil {
		acc.ExcludedFromNetWorth = !*body.IncludeInNetWorth
	}
	if body.InterestRate != nil {
		acc.InterestRate = floatPtr(body.InterestRate)
	}
	if body.TermMonths != nil {
		acc.TermMonths = body.TermMonths
	}
	if body.MaturityDate != nil {
		acc.MaturityDate = datePtr(body.MaturityDate)
	}
	if body.Capitalization != nil {
		acc.Capitalization = body.Capitalization
	}
	if body.CreditLimit != nil {
		acc.CreditLimit = optMoney(body.CreditLimit, acc.Currency)
	}
	if body.Principal != nil {
		acc.Principal = optMoney(body.Principal, acc.Currency)
	}
	if body.StartDate != nil {
		acc.StartDate = datePtr(body.StartDate)
	}
	if body.PaymentDay != nil {
		acc.PaymentDay = body.PaymentDay
	}
	if body.CardLast4 != nil {
		acc.CardLast4 = body.CardLast4
	}

	if err = s.accounts.Update(ctx, acc); err != nil {
		s.logger.Error("update account", zap.Error(err))

		return nil, err
	}

	balance, err := s.accountBalance(ctx, acc.ID, *acc)
	if err != nil {
		return nil, err
	}

	return api.UpdateAccount200JSONResponse(toAccount(*acc, balance)), nil
}

func (s Server) DeleteAccount(ctx context.Context, request api.DeleteAccountRequestObject) (api.DeleteAccountResponseObject, error) {
	err := s.accounts.Delete(ctx, request.Id)
	switch {
	case errors.Is(err, accountrepository.ErrNotFound):
		return api.DeleteAccount404JSONResponse{NotFoundJSONResponse: notFound("account not found")}, nil
	case errors.Is(err, accountrepository.ErrInUse):
		return api.DeleteAccount409JSONResponse{Error: "account has transactions; archive it instead"}, nil
	case err != nil:
		return nil, err
	}

	return api.DeleteAccount204Response{}, nil
}

func (s Server) GetAmortization(ctx context.Context, request api.GetAmortizationRequestObject) (api.GetAmortizationResponseObject, error) {
	acc, err := s.accounts.Get(ctx, request.Id)
	if errors.Is(err, accountrepository.ErrNotFound) {
		return api.GetAmortization404JSONResponse{NotFoundJSONResponse: notFound("account not found")}, nil
	}
	if err != nil {
		return nil, err
	}

	if acc.Type != entities.TypeLoan {
		return api.GetAmortization400JSONResponse{BadRequestJSONResponse: badRequest("account is not a loan")}, nil
	}
	if acc.Principal == nil || acc.InterestRate == nil || acc.TermMonths == nil || acc.StartDate == nil {
		return api.GetAmortization400JSONResponse{BadRequestJSONResponse: badRequest("loan terms are incomplete (need principal, interest_rate, term_months, start_date)")}, nil
	}

	paymentDay := 0
	if acc.PaymentDay != nil {
		paymentDay = *acc.PaymentDay
	}

	sched, err := ledger.Amortize(*acc.Principal, *acc.InterestRate, *acc.TermMonths, *acc.StartDate, paymentDay)
	if err != nil {
		return api.GetAmortization400JSONResponse{BadRequestJSONResponse: badRequest(err.Error())}, nil
	}

	return api.GetAmortization200JSONResponse(toAmortization(acc.Currency, sched)), nil
}

func toAmortization(currency string, sched ledger.AmortizationSchedule) api.AmortizationSchedule {
	rows := make([]api.AmortizationRow, len(sched.Rows))
	for i, r := range sched.Rows {
		rows[i] = api.AmortizationRow{
			Period:    r.Period,
			Date:      openapi_types.Date{Time: r.Date},
			Payment:   r.Payment,
			Principal: r.Principal,
			Interest:  r.Interest,
			Balance:   r.Balance,
		}
	}

	return api.AmortizationSchedule{
		Currency:       currency,
		MonthlyPayment: sched.MonthlyPayment,
		TotalPayment:   sched.TotalPayment,
		TotalInterest:  sched.TotalInterest,
		Rows:           rows,
	}
}

// accountBalance fetches the single account's derived balance.
func (s Server) accountBalance(ctx context.Context, id uuid.UUID, acc entities.Account) (money.Money, error) {
	balances, err := s.accounts.Balances(ctx)
	if err != nil {
		return money.Money{}, err
	}

	if b, ok := balances[id]; ok {
		return b, nil
	}

	return ledgerOpening(acc), nil
}

func ledgerOpening(acc entities.Account) money.Money {
	// no transactions → balance equals opening balance for both kinds
	return acc.OpeningBalance
}

func optMoney(v *int64, currency string) *money.Money {
	if v == nil {
		return nil
	}
	m := money.New(*v, currency)

	return &m
}

func toAccount(acc entities.Account, balance money.Money) api.Account {
	a := api.Account{
		Id:                acc.ID,
		Name:              acc.Name,
		Kind:              api.AccountKind(acc.Kind),
		Type:              api.AccountType(acc.Type),
		Currency:          acc.Currency,
		OpeningBalance:    acc.OpeningBalance.Minor(),
		Balance:           balance,
		Archived:          acc.Archived,
		IncludeInNetWorth: !acc.ExcludedFromNetWorth,
		CreatedAt:         acc.CreatedAt,
	}

	if acc.InterestRate != nil {
		a.InterestRate = nullable.NewNullableWithValue(float32(*acc.InterestRate))
	}
	if acc.TermMonths != nil {
		a.TermMonths = nullable.NewNullableWithValue(*acc.TermMonths)
	}
	if acc.MaturityDate != nil {
		a.MaturityDate = nullable.NewNullableWithValue(openapi_types.Date{Time: *acc.MaturityDate})
	}
	if acc.Capitalization != nil {
		a.Capitalization = nullable.NewNullableWithValue(*acc.Capitalization)
	}
	if acc.CreditLimit != nil {
		a.CreditLimit = nullable.NewNullableWithValue(acc.CreditLimit.Minor())
	}
	if acc.Principal != nil {
		a.Principal = nullable.NewNullableWithValue(acc.Principal.Minor())
	}
	if acc.StartDate != nil {
		a.StartDate = nullable.NewNullableWithValue(openapi_types.Date{Time: *acc.StartDate})
	}
	if acc.PaymentDay != nil {
		a.PaymentDay = nullable.NewNullableWithValue(*acc.PaymentDay)
	}
	if acc.CardLast4 != nil {
		a.CardLast4 = nullable.NewNullableWithValue(*acc.CardLast4)
	}

	return a
}

func floatPtr(v *float32) *float64 {
	if v == nil {
		return nil
	}
	f := float64(*v)

	return &f
}

func datePtr(d *openapi_types.Date) *time.Time {
	if d == nil {
		return nil
	}

	return &d.Time
}
