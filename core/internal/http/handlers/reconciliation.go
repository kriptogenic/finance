package handlers

import (
	"context"

	"github.com/oapi-codegen/nullable"
	"go.uber.org/zap"

	"finance/generated/api"
	"finance/internal/entities"
	"finance/internal/ledger"
	"finance/pkg/money"
)

func (s Server) IngestBalances(ctx context.Context, request api.IngestBalancesRequestObject) (api.IngestBalancesResponseObject, error) {
	if request.Body == nil {
		return api.IngestBalances400JSONResponse{BadRequestJSONResponse: badRequest("empty body")}, nil
	}
	body := request.Body

	for _, e := range body.Balances {
		snap := entities.BalanceSnapshot{
			CardLast4:  e.CardLast4,
			Bank:       e.Bank,
			Amount:     money.New(e.Amount, e.Currency),
			Source:     body.Source,
			ReportedAt: body.ReportedAt,
		}
		if err := s.snapshots.Upsert(ctx, &snap); err != nil {
			s.logger.Error("upsert balance snapshot", zap.Error(err))

			return nil, err
		}
	}

	return api.IngestBalances204Response{}, nil
}

func (s Server) GetReconciliation(ctx context.Context, _ api.GetReconciliationRequestObject) (api.GetReconciliationResponseObject, error) {
	snaps, err := s.snapshots.List(ctx)
	if err != nil {
		return nil, err
	}

	accounts, err := s.accounts.List(ctx, false)
	if err != nil {
		return nil, err
	}

	balances, err := s.accounts.Balances(ctx)
	if err != nil {
		return nil, err
	}

	recon := ledger.Reconcile(snaps, accounts, balances)

	rows := make([]api.ReconciliationRow, len(recon))
	for i, r := range recon {
		row := api.ReconciliationRow{
			CardLast4:        r.Snapshot.CardLast4,
			AccountId:        r.Account.ID,
			AccountName:      r.Account.Name,
			Reported:         r.Snapshot.Amount,
			Derived:          r.Derived,
			InSync:           r.InSync,
			CurrencyMismatch: !r.CurrencyMatch,
			ReportedAt:       r.Snapshot.ReportedAt,
		}
		if r.Snapshot.Bank != nil {
			row.Bank = nullable.NewNullableWithValue(*r.Snapshot.Bank)
		}
		if r.CurrencyMatch {
			delta := r.Delta
			row.Delta = &delta
		}

		rows[i] = row
	}

	return api.GetReconciliation200JSONResponse{Base: s.base, Rows: rows}, nil
}
