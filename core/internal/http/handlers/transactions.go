package handlers

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/oapi-codegen/nullable"
	"go.uber.org/zap"

	"finance/generated/api"
	"finance/internal/entities"
	"finance/internal/ledger"
	"finance/internal/pushnotify"
	accountrepository "finance/internal/repositories/account_repository"
	categoryrepository "finance/internal/repositories/category_repository"
	transactionrepository "finance/internal/repositories/transaction_repository"
	"finance/pkg/fx"
	"finance/pkg/money"
)

func (s Server) CreateTransaction(ctx context.Context, request api.CreateTransactionRequestObject) (api.CreateTransactionResponseObject, error) {
	if request.Body == nil {
		return api.CreateTransaction400JSONResponse{BadRequestJSONResponse: badRequest("empty body")}, nil
	}

	tx, msg := s.buildTransaction(ctx, request.Body)
	if msg != "" {
		return api.CreateTransaction400JSONResponse{BadRequestJSONResponse: badRequest(msg)}, nil
	}

	if err := s.transactions.Create(ctx, &tx); err != nil {
		s.logger.Error("create transaction", zap.Error(err))

		return nil, err
	}

	return api.CreateTransaction201JSONResponse(s.toTransaction(tx)), nil
}

func (s Server) UpdateTransaction(ctx context.Context, request api.UpdateTransactionRequestObject) (api.UpdateTransactionResponseObject, error) {
	existing, err := s.transactions.Get(ctx, request.Id)
	if errors.Is(err, transactionrepository.ErrNotFound) {
		return api.UpdateTransaction404JSONResponse{NotFoundJSONResponse: notFound("transaction not found")}, nil
	}
	if err != nil {
		return nil, err
	}

	if request.Body == nil {
		return api.UpdateTransaction400JSONResponse{BadRequestJSONResponse: badRequest("empty body")}, nil
	}

	// full replace: rebuild through the engine so the edit re-validates and
	// re-freezes the rate, then keep the original id/created_at.
	tx, msg := s.buildTransaction(ctx, request.Body)
	if msg != "" {
		return api.UpdateTransaction400JSONResponse{BadRequestJSONResponse: badRequest(msg)}, nil
	}
	tx.ID = existing.ID
	tx.CreatedAt = existing.CreatedAt

	if err = s.transactions.Update(ctx, &tx); err != nil {
		if errors.Is(err, transactionrepository.ErrNotFound) {
			return api.UpdateTransaction404JSONResponse{NotFoundJSONResponse: notFound("transaction not found")}, nil
		}
		s.logger.Error("update transaction", zap.Error(err))

		return nil, err
	}

	return api.UpdateTransaction200JSONResponse(s.toTransaction(tx)), nil
}

// buildTransaction resolves the referenced buckets and runs the engine. It
// returns a non-empty message describing the first client error, or an empty
// string on success.
func (s Server) buildTransaction(ctx context.Context, body *api.CreateTransactionRequest) (entities.Transaction, string) {
	in := ledgerInput(body)

	if body.FromAccountId != nil {
		acc, msg := s.getAccount(ctx, *body.FromAccountId)
		if msg != "" {
			return entities.Transaction{}, msg
		}
		in.From = acc
	}
	if body.ToAccountId != nil {
		acc, msg := s.getAccount(ctx, *body.ToAccountId)
		if msg != "" {
			return entities.Transaction{}, msg
		}
		in.To = acc
	}
	if body.CategoryId != nil {
		c, msg := s.getCategory(ctx, *body.CategoryId)
		if msg != "" {
			return entities.Transaction{}, msg
		}
		in.Category = c
	}
	if body.RateToBase != nil {
		rate, err := fx.ParseRate(*body.RateToBase)
		if err != nil {
			return entities.Transaction{}, "invalid rate_to_base"
		}
		in.RateToBase = &rate
	}

	tx, err := ledger.BuildTransaction(in, s.base)
	if err != nil {
		return entities.Transaction{}, err.Error()
	}

	return tx, ""
}

func (s Server) ListTransactions(ctx context.Context, request api.ListTransactionsRequestObject) (api.ListTransactionsResponseObject, error) {
	p := request.Params
	filter := transactionrepository.Filter{
		AccountID:  p.AccountId,
		CategoryID: p.CategoryId,
		DateFrom:   p.DateFrom,
		DateTo:     p.DateTo,
		Tag:        p.Tag,
		Query:      p.Q,
	}
	if p.Type != nil {
		t := entities.TransactionType(*p.Type)
		filter.Type = &t
	}
	if p.Uncategorized != nil {
		filter.Uncategorized = *p.Uncategorized
	}
	if p.Limit != nil {
		filter.Limit = *p.Limit
	}
	if p.Offset != nil {
		filter.Offset = *p.Offset
	}

	txns, err := s.transactions.List(ctx, filter)
	if err != nil {
		return nil, err
	}

	out := make([]api.Transaction, len(txns))
	for i, t := range txns {
		out[i] = s.toTransaction(t)
	}

	return api.ListTransactions200JSONResponse{Transactions: out}, nil
}

func (s Server) GetTransaction(ctx context.Context, request api.GetTransactionRequestObject) (api.GetTransactionResponseObject, error) {
	tx, err := s.transactions.Get(ctx, request.Id)
	if errors.Is(err, transactionrepository.ErrNotFound) {
		return api.GetTransaction404JSONResponse{NotFoundJSONResponse: notFound("transaction not found")}, nil
	}
	if err != nil {
		return nil, err
	}

	return api.GetTransaction200JSONResponse(s.toTransaction(*tx)), nil
}

func (s Server) IngestTransaction(ctx context.Context, request api.IngestTransactionRequestObject) (api.IngestTransactionResponseObject, error) {
	if request.Body == nil {
		return api.IngestTransaction400JSONResponse{BadRequestJSONResponse: badRequest("empty body")}, nil
	}
	body := request.Body

	create := api.CreateTransactionRequest{
		Date:       body.Date,
		Type:       body.Type,
		Amount:     body.Amount,
		ToAmount:   body.ToAmount,
		RateToBase: body.RateToBase,
		Note:       body.Merchant, // raw merchant text is stored as the note
		Tags:       body.Tags,
	}

	// the app owns routing: resolve cards -> accounts
	if body.FromCardLast4 != nil {
		id, resp := s.resolveCard(ctx, *body.FromCardLast4)
		if resp != nil {
			return resp, nil
		}
		create.FromAccountId = id
	}
	if body.ToCardLast4 != nil {
		id, resp := s.resolveCard(ctx, *body.ToCardLast4)
		if resp != nil {
			return resp, nil
		}
		create.ToAccountId = id
	}

	// ... and the merchant -> a category (rule, else the Uncategorized bucket)
	if body.Type != api.TransactionTypeTransfer {
		catType := entities.CategoryExpense
		if body.Type == api.TransactionTypeIncome {
			catType = entities.CategoryIncome
		}
		merchant := ""
		if body.Merchant != nil {
			merchant = *body.Merchant
		}
		catID, err := s.categories.ResolveForIngest(ctx, catType, merchant)
		if err != nil {
			s.logger.Error("resolve ingest category", zap.Error(err))

			return api.IngestTransaction400JSONResponse{BadRequestJSONResponse: badRequest("could not resolve a category (seed the Uncategorized buckets)")}, nil
		}
		create.CategoryId = &catID
	}

	tx, msg := s.buildTransaction(ctx, &create)
	if msg != "" {
		return api.IngestTransaction400JSONResponse{BadRequestJSONResponse: badRequest(msg)}, nil
	}

	tx.ExternalID = &body.ExternalId
	tx.TransferGroupID = body.TransferGroupId

	created, err := s.transactions.Ingest(ctx, &tx)
	if err != nil {
		s.logger.Error("ingest transaction", zap.Error(err))

		return nil, err
	}

	if created {
		// A newly-ingested expense/income may have landed in the Uncategorized
		// bucket — refresh the PWA badge across devices (no-op if push is off).
		if create.CategoryId != nil {
			merchant := ""
			if tx.Note != nil {
				merchant = *tx.Note
			}
			s.notifier.OnIngestedCategory(pushnotify.Ingested{
				CategoryID: *create.CategoryId,
				Merchant:   merchant,
				Amount:     money.New(tx.Amount, tx.Currency).Display(),
			})
		}

		return api.IngestTransaction201JSONResponse(s.toTransaction(tx)), nil
	}

	return api.IngestTransaction200JSONResponse(s.toTransaction(tx)), nil
}

// resolveCard maps a card last4 to its account id, or returns a 400 response.
func (s Server) resolveCard(ctx context.Context, last4 string) (*uuid.UUID, api.IngestTransactionResponseObject) {
	acc, err := s.accounts.ByCardLast4(ctx, last4)
	if errors.Is(err, accountrepository.ErrNotFound) {
		return nil, api.IngestTransaction400JSONResponse{BadRequestJSONResponse: badRequest("unknown card: " + last4)}
	}
	if err != nil {
		s.logger.Error("resolve card", zap.Error(err))

		return nil, api.IngestTransaction400JSONResponse{BadRequestJSONResponse: badRequest("card resolution failed")}
	}

	return &acc.ID, nil
}

func (s Server) PatchTransaction(ctx context.Context, request api.PatchTransactionRequestObject) (api.PatchTransactionResponseObject, error) {
	if request.Body == nil {
		return api.PatchTransaction400JSONResponse{BadRequestJSONResponse: badRequest("empty body")}, nil
	}

	tx, err := s.transactions.Get(ctx, request.Id)
	if errors.Is(err, transactionrepository.ErrNotFound) {
		return api.PatchTransaction404JSONResponse{NotFoundJSONResponse: notFound("transaction not found")}, nil
	}
	if err != nil {
		return nil, err
	}

	if tx.Type == entities.TxTransfer {
		return api.PatchTransaction400JSONResponse{BadRequestJSONResponse: badRequest("transfers have no category")}, nil
	}

	cat, msg := s.getCategory(ctx, request.Body.CategoryId)
	if msg != "" {
		return api.PatchTransaction400JSONResponse{BadRequestJSONResponse: badRequest(msg)}, nil
	}
	if cat.Archived {
		return api.PatchTransaction400JSONResponse{BadRequestJSONResponse: badRequest("category is archived")}, nil
	}
	if !categoryMatchesTx(cat.Type, tx.Type) {
		return api.PatchTransaction400JSONResponse{BadRequestJSONResponse: badRequest("category type does not match the transaction type")}, nil
	}

	if err = s.transactions.SetCategory(ctx, tx.ID, cat.ID); err != nil {
		if errors.Is(err, transactionrepository.ErrNotFound) {
			return api.PatchTransaction404JSONResponse{NotFoundJSONResponse: notFound("transaction not found")}, nil
		}
		s.logger.Error("set transaction category", zap.Error(err))

		return nil, err
	}

	tx.CategoryID = &cat.ID

	return api.PatchTransaction200JSONResponse(s.toTransaction(*tx)), nil
}

func (s Server) GetCategorySuggestions(ctx context.Context, request api.GetCategorySuggestionsRequestObject) (api.GetCategorySuggestionsResponseObject, error) {
	tx, err := s.transactions.Get(ctx, request.Id)
	if errors.Is(err, transactionrepository.ErrNotFound) {
		return api.GetCategorySuggestions404JSONResponse{NotFoundJSONResponse: notFound("transaction not found")}, nil
	}
	if err != nil {
		return nil, err
	}

	catType, ok := categoryTypeFor(tx.Type)
	if !ok {
		// transfers carry no category — nothing to suggest
		return api.GetCategorySuggestions200JSONResponse{Suggestions: []api.CategorySuggestion{}}, nil
	}

	merchant := ""
	if tx.Note != nil {
		merchant = *tx.Note
	}

	cats, err := s.categories.List(ctx, &catType, false)
	if err != nil {
		return nil, err
	}

	// candidate pool: real (non-system) categories of this type
	byName := make(map[string]entities.Category, len(cats))
	names := make([]string, 0, len(cats))
	for _, c := range cats {
		if c.SystemKey != nil {
			continue
		}
		byName[c.Name] = c
		names = append(names, c.Name)
	}

	var suggestions []api.CategorySuggestion
	seen := make(map[uuid.UUID]struct{})
	add := func(c entities.Category, source api.SuggestionSource) {
		if _, dup := seen[c.ID]; dup {
			return
		}
		seen[c.ID] = struct{}{}
		suggestions = append(suggestions, api.CategorySuggestion{
			CategoryId:   c.ID,
			CategoryName: c.Name,
			Source:       source,
		})
	}

	// a matching local rule wins the top spot
	if ruleID, err := s.categories.MatchRule(ctx, catType, merchant); err != nil {
		s.logger.Error("match category rule", zap.Error(err))
	} else if ruleID != nil {
		for _, c := range cats {
			if c.ID == *ruleID {
				add(c, api.Rule)

				break
			}
		}
	}

	// then the LLM ranks the user's existing categories
	ranked, err := s.catSuggest.Suggest(ctx, merchant, names)
	if err != nil {
		s.logger.Warn("category suggestion failed", zap.Error(err))
	}
	for _, name := range ranked {
		if c, ok := byName[name]; ok {
			add(c, api.Ai)
		}
	}

	if suggestions == nil {
		suggestions = []api.CategorySuggestion{}
	}

	return api.GetCategorySuggestions200JSONResponse{Suggestions: suggestions}, nil
}

// categoryTypeFor maps a transaction type to the category tree it draws from.
func categoryTypeFor(t entities.TransactionType) (entities.CategoryType, bool) {
	switch t {
	case entities.TxExpense:
		return entities.CategoryExpense, true
	case entities.TxIncome:
		return entities.CategoryIncome, true
	default:
		return "", false
	}
}

func categoryMatchesTx(ct entities.CategoryType, tt entities.TransactionType) bool {
	want, ok := categoryTypeFor(tt)

	return ok && want == ct
}

func (s Server) DeleteTransaction(ctx context.Context, request api.DeleteTransactionRequestObject) (api.DeleteTransactionResponseObject, error) {
	err := s.transactions.Delete(ctx, request.Id)
	if errors.Is(err, transactionrepository.ErrNotFound) {
		return api.DeleteTransaction404JSONResponse{NotFoundJSONResponse: notFound("transaction not found")}, nil
	}
	if err != nil {
		return nil, err
	}

	return api.DeleteTransaction204Response{}, nil
}

func ledgerInput(body *api.CreateTransactionJSONRequestBody) ledger.NewTransaction {
	date := time.Now()
	if body.Date != nil {
		date = *body.Date
	}

	var tags []string
	if body.Tags != nil {
		tags = *body.Tags
	}

	return ledger.NewTransaction{
		Date:     date,
		Type:     entities.TransactionType(body.Type),
		Amount:   body.Amount,
		ToAmount: body.ToAmount,
		Note:     body.Note,
		Tags:     tags,
	}
}

func (s Server) getAccount(ctx context.Context, id uuid.UUID) (*entities.Account, string) {
	acc, err := s.accounts.Get(ctx, id)
	if errors.Is(err, accountrepository.ErrNotFound) {
		return nil, "account not found: " + id.String()
	}
	if err != nil {
		s.logger.Error("load account", zap.Error(err))

		return nil, "invalid account"
	}

	return acc, ""
}

func (s Server) getCategory(ctx context.Context, id uuid.UUID) (*entities.Category, string) {
	c, err := s.categories.Get(ctx, id)
	if errors.Is(err, categoryrepository.ErrNotFound) {
		return nil, "category not found: " + id.String()
	}
	if err != nil {
		s.logger.Error("load category", zap.Error(err))

		return nil, "invalid category"
	}

	return c, ""
}

func (s Server) toTransaction(tx entities.Transaction) api.Transaction {
	tags := tx.Tags
	if tags == nil {
		tags = []string{}
	}

	out := api.Transaction{
		Id:        tx.ID,
		Date:      tx.Date,
		Type:      api.TransactionType(tx.Type),
		Amount:    money.New(tx.Amount, tx.Currency),
		Tags:      tags,
		CreatedAt: tx.CreatedAt,
	}

	if tx.FromAccountID != nil {
		out.FromAccountId = nullable.NewNullableWithValue(*tx.FromAccountID)
	}
	if tx.ToAccountID != nil {
		out.ToAccountId = nullable.NewNullableWithValue(*tx.ToAccountID)
	}
	if tx.CategoryID != nil {
		out.CategoryId = nullable.NewNullableWithValue(*tx.CategoryID)
	}
	if tx.ToAmount != nil && tx.ToCurrency != nil {
		m := money.New(*tx.ToAmount, *tx.ToCurrency)
		out.ToAmount = &m
	}
	if tx.RateToBase != nil {
		out.RateToBase = nullable.NewNullableWithValue(tx.RateToBase.String())
	}
	if tx.BaseAmount != nil {
		m := money.New(*tx.BaseAmount, s.base)
		out.BaseAmount = &m
	}
	if tx.Note != nil {
		out.Note = nullable.NewNullableWithValue(*tx.Note)
	}
	if tx.ExternalID != nil {
		out.ExternalId = nullable.NewNullableWithValue(*tx.ExternalID)
	}
	if tx.TransferGroupID != nil {
		out.TransferGroupId = nullable.NewNullableWithValue(*tx.TransferGroupID)
	}

	return out
}
