package delivery

import (
	"time"

	"finance/lookout/generated/core"
	"finance/lookout/internal/pairing"
	"finance/lookout/internal/parser"
)

func toRequest(p pairing.Posting) core.IngestTransactionRequest {
	req := core.IngestTransactionRequest{
		ExternalId: p.ExternalID,
		Type:       core.TransactionType(p.Type),
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

func toBalanceRequest(balances []parser.CardBalance, reportedAt time.Time) core.BalanceSnapshotRequest {
	source := "humo"
	req := core.BalanceSnapshotRequest{
		ReportedAt: reportedAt,
		Source:     &source,
		Balances:   make([]core.BalanceSnapshotEntry, 0, len(balances)),
	}
	for _, b := range balances {
		entry := core.BalanceSnapshotEntry{
			CardLast4: b.CardLast4,
			Amount:    b.Amount,
			Currency:  b.Currency,
		}
		if b.Bank != "" {
			bank := b.Bank
			entry.Bank = &bank
		}
		req.Balances = append(req.Balances, entry)
	}
	return req
}
