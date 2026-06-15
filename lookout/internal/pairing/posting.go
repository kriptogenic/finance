package pairing

import "time"

// Posting is the bot's neutral description of one transaction to ingest — the
// result of mapping parsed notification(s) onto the finance app's bucket model
// (§5). The pairing stage produces it because it is the only stage with the
// temporal context to decide expense vs. income vs. transfer; the delivery stage
// translates it to the generated IngestTransactionRequest and POSTs it. Money is
// int64 minor units; never float (§12).
type Posting struct {
	// ExternalID is the idempotency key (§7): tg:<chat>:<msg> for a standalone
	// leg, tg:transfer:<lo>-<hi> for a collapsed transfer.
	ExternalID string

	Type string // "expense" | "income" | "transfer"

	// FromCardLast4 is set when money leaves a card (expense, transfer).
	FromCardLast4 string
	// ToCardLast4 is set when money enters a card (income, transfer).
	ToCardLast4 string

	// Merchant is the raw notification text the app routes to a category and
	// stores as a note. Empty for transfers, which carry no category (§5.1).
	Merchant string

	Amount int64     // minor units
	Date   time.Time // notification 🕓 time, TZ-aware

	// TransferGroupID links the legs of a transfer; empty for standalone legs.
	TransferGroupID string

	Tags []string
}
