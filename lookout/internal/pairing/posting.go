package pairing

import "time"

type Posting struct {
	ExternalID string

	Type string

	FromCardLast4 string

	ToCardLast4 string

	Merchant string

	Amount int64
	Date   time.Time

	TransferGroupID string

	Tags []string
}
