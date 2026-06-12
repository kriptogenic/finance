// Package money decorates Rhymond/go-money with JSON (de)serialization so the
// type can be wired straight into generated OpenAPI models via x-go-type. The
// underlying value stays an integer minor-unit amount plus an ISO-4217 currency
// code — never floating point (REQUIREMENTS §3).
package money

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	gomoney "github.com/Rhymond/go-money"
)

// Money embeds *go-money.Money, so the whole go-money API (Add, Subtract,
// Display, Compare, ...) is available on the wrapper. It serializes as
//
//	{"amount": <int64 minor units>, "currency": "<ISO-4217>"}
type Money struct {
	*gomoney.Money
}

// New builds a Money from an integer minor-unit amount and an ISO-4217 code.
func New(amount int64, code string) Money {
	return Money{gomoney.New(amount, code)}
}

// Wrap adapts an existing *go-money.Money (e.g. a ledger result) into the wrapper.
func Wrap(m *gomoney.Money) Money {
	return Money{m}
}

// IsZeroValue reports whether the wrapper holds no money value at all (as
// opposed to a zero amount).
func (m Money) IsZeroValue() bool {
	return m.Money == nil
}

type payload struct {
	Amount   *int64 `json:"amount"`
	Currency string `json:"currency"`
}

// MarshalJSON implements json.Marshaler.
func (m Money) MarshalJSON() ([]byte, error) {
	if m.Money == nil {
		return []byte("null"), nil
	}

	amount := m.Amount()

	return json.Marshal(payload{Amount: &amount, Currency: m.Currency().Code})
}

// UnmarshalJSON implements json.Unmarshaler.
func (m *Money) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		m.Money = nil

		return nil
	}

	var p payload
	if err := json.Unmarshal(data, &p); err != nil {
		return fmt.Errorf("money: decode: %w", err)
	}

	if p.Amount == nil {
		return errors.New("money: missing amount")
	}

	if p.Currency == "" {
		return errors.New("money: missing currency")
	}

	m.Money = gomoney.New(*p.Amount, p.Currency)

	return nil
}

// Value implements driver.Valuer, rendering the value as a PostgreSQL composite
// literal "(amount,currency)" — suitable for a column of a composite type whose
// fields are (bigint, text). A nil value stores SQL NULL.
func (m Money) Value() (driver.Value, error) {
	if m.Money == nil {
		return nil, nil
	}

	return fmt.Sprintf("(%d,%s)", m.Amount(), m.Currency().Code), nil
}

// Scan implements sql.Scanner for the same "(amount,currency)" composite text.
func (m *Money) Scan(src any) error {
	if src == nil {
		m.Money = nil

		return nil
	}

	var raw string
	switch v := src.(type) {
	case string:
		raw = v
	case []byte:
		raw = string(v)
	default:
		return fmt.Errorf("money: cannot scan %T", src)
	}

	amount, code, err := parseComposite(raw)
	if err != nil {
		return err
	}

	m.Money = gomoney.New(amount, code)

	return nil
}

func parseComposite(raw string) (int64, string, error) {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "(")
	s = strings.TrimSuffix(s, ")")

	parts := strings.SplitN(s, ",", 2)
	if len(parts) != 2 {
		return 0, "", fmt.Errorf("money: invalid composite %q", raw)
	}

	amount, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
	if err != nil {
		return 0, "", fmt.Errorf("money: amount: %w", err)
	}

	code := strings.Trim(strings.TrimSpace(parts[1]), `"`)
	if code == "" {
		return 0, "", errors.New("money: missing currency")
	}

	return amount, code, nil
}
