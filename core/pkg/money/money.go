package money

import (
	"bytes"
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	gomoney "github.com/Rhymond/go-money"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type Money struct {
	*gomoney.Money
	scan *scanState // non-nil only mid composite decode
}

func New(amount int64, code string) Money {
	return Money{Money: gomoney.New(amount, code)}
}

func Wrap(m *gomoney.Money) Money {
	return Money{Money: m}
}

func Zero(code string) Money {
	return Money{Money: gomoney.New(0, code)}
}

func (m Money) IsZeroValue() bool {
	return m.Money == nil
}

func (m Money) Minor() int64 { return m.Amount() }

func (m Money) Code() string { return m.Currency().Code }

func (m Money) Plus(o Money) (Money, error) {
	sum, err := m.Add(o.Money)
	if err != nil {
		return Money{}, err
	}

	return Wrap(sum), nil
}

func (m Money) Minus(o Money) (Money, error) {
	diff, err := m.Subtract(o.Money)
	if err != nil {
		return Money{}, err
	}

	return Wrap(diff), nil
}

func (m Money) Neg() Money { return Wrap(m.Negative()) }

// CompositeTypeName is the PostgreSQL composite type Money maps to: (amount bigint, currency text).
const CompositeTypeName = "money_t"

// RegisterType loads money_t and registers it on the connection so pgx can
// encode/scan Money values. Wire it into pgxpool's AfterConnect.
func RegisterType(ctx context.Context, conn *pgx.Conn) error {
	t, err := conn.LoadType(ctx, CompositeTypeName)
	if err != nil {
		return fmt.Errorf("money: load %s: %w", CompositeTypeName, err)
	}
	conn.TypeMap().RegisterType(t)

	return nil
}

// --- pgx composite codec (pgtype.CompositeIndexGetter / CompositeIndexScanner) ---

type scanState struct{ amount int64 }

// IsNull implements pgtype.CompositeIndexGetter.
func (m Money) IsNull() bool { return m.Money == nil }

// Index implements pgtype.CompositeIndexGetter: field 0 = amount, 1 = currency.
func (m Money) Index(i int) any {
	switch i {
	case 0:
		return m.Amount()
	case 1:
		return m.Currency().Code
	}

	return nil
}

// ScanNull implements pgtype.CompositeIndexScanner.
func (m *Money) ScanNull() error {
	m.Money = nil
	m.scan = nil

	return nil
}

// ScanIndex implements pgtype.CompositeIndexScanner. The amount (field 0) is
// buffered until the currency (field 1) arrives, then the value is assembled.
func (m *Money) ScanIndex(i int) any {
	if m.scan == nil {
		m.scan = &scanState{}
	}
	switch i {
	case 0:
		return &m.scan.amount
	case 1:
		return &codeScanner{m: m}
	}

	return nil
}

type codeScanner struct{ m *Money }

// ScanText implements pgtype.TextScanner for the currency field.
func (s *codeScanner) ScanText(v pgtype.Text) error {
	if !v.Valid {
		return errors.New("money: null currency in composite")
	}
	s.m.Money = gomoney.New(s.m.scan.amount, v.String)
	s.m.scan = nil

	return nil
}

type payload struct {
	Amount   *int64 `json:"amount"`
	Currency string `json:"currency"`
}

func (m Money) MarshalJSON() ([]byte, error) {
	if m.Money == nil {
		return []byte("null"), nil
	}

	amount := m.Amount()

	return json.Marshal(payload{Amount: &amount, Currency: m.Currency().Code})
}

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

func (m Money) Value() (driver.Value, error) {
	if m.Money == nil {
		return nil, nil
	}

	return fmt.Sprintf("(%d,%s)", m.Amount(), m.Currency().Code), nil
}

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
