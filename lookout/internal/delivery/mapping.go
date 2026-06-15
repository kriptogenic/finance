package delivery

import (
	"finance/lookout/generated/api"
	"finance/lookout/internal/pairing"
)

// toRequest maps the bot's neutral Posting onto the generated ingest request
// (§5, §7). Card→account and merchant→category are resolved server-side, so we
// send only card last4s and the raw merchant. rate_to_base is omitted: all
// notifications are UZS and the app's base is UZS (§5.2). balance_after, raw
// text, and the parsed flag are bot-side only and never sent (§7).
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
	if p.Merchant != "" { // transfers carry none (§5.1)
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
