package delivery

import (
	"finance/lookout/generated/api"
	"finance/lookout/internal/pairing"
)

func toRequest(p pairing.Posting) api.IngestTransactionRequest {
	req := api.IngestTransactionRequest{
		ExternalId: p.ExternalID,
		Type:       api.TransactionType(p.Type),
		Amount:     p.Amount,
	}
	if !p.Date.IsZero() {
		d := p.Date
		req.Date = &d
	}
	if p.FromCardLast4 != "" {
		req.FromCardLast4 = &p.FromCardLast4
	}
	if p.ToCardLast4 != "" {
		req.ToCardLast4 = &p.ToCardLast4
	}
	if p.Merchant != "" {
		req.Merchant = &p.Merchant
	}
	if p.TransferGroupID != "" {
		req.TransferGroupId = &p.TransferGroupID
	}
	if len(p.Tags) > 0 {
		tags := append([]string(nil), p.Tags...)
		req.Tags = &tags
	}
	return req
}
