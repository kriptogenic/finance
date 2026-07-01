package entities

import (
	"time"

	"github.com/google/uuid"

	"finance/pkg/money"
)

// LoanSchedule is one persisted installment of a loan's amortization plan. Rows
// are seeded from the annuity generator and then hold the real payment date
// (weekend/holiday roll or a manual override) and the interest/principal split
// that date implies. All money is in the loan account's currency.
type LoanSchedule struct {
	ID        uuid.UUID `db:"id"`
	AccountID uuid.UUID `db:"account_id"`
	Period    int       `db:"period"`

	DueDate      time.Time  `db:"due_date"`      // effective date: nominal, rolled, or overridden
	DateOverride *time.Time `db:"date_override"` // manual edge-case date; wins over the calendar

	Payment   money.Money `db:"payment"` // = Principal + Interest
	Principal money.Money `db:"principal"`
	Interest  money.Money `db:"interest"`
	Balance   money.Money `db:"balance"` // remaining principal after this installment

	Paid      bool      `db:"paid"`
	CreatedAt time.Time `db:"created_at"`
}

// Holiday is a public non-working day that pushes a loan payment to the next
// business day.
type Holiday struct {
	Day  time.Time `db:"day"`
	Name string    `db:"name"`
}
