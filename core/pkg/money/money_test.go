package money_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"finance/pkg/money"
)

func TestMarshalJSON_Shape(t *testing.T) {
	b, err := json.Marshal(money.New(100000, "USD"))
	require.NoError(t, err)
	require.JSONEq(t, `{"amount":100000,"currency":"USD"}`, string(b))
}

func TestMarshalJSON_NilIsNull(t *testing.T) {
	b, err := json.Marshal(money.Money{})
	require.NoError(t, err)
	require.Equal(t, "null", string(b))
}

func TestUnmarshalJSON_RoundTrip(t *testing.T) {
	var m money.Money
	require.NoError(t, json.Unmarshal([]byte(`{"amount":-2500,"currency":"UZS"}`), &m))
	require.Equal(t, int64(-2500), m.Amount())
	require.Equal(t, "UZS", m.Currency().Code)

	// embedded go-money API stays usable through the wrapper
	other := money.New(500, "UZS")
	sum, err := m.Add(other.Money)
	require.NoError(t, err)
	require.Equal(t, int64(-2000), sum.Amount())
}

func TestUnmarshalJSON_Errors(t *testing.T) {
	var m money.Money
	require.Error(t, json.Unmarshal([]byte(`{"currency":"USD"}`), &m), "missing amount")
	require.Error(t, json.Unmarshal([]byte(`{"amount":100}`), &m), "missing currency")
}

func TestUnmarshalJSON_Null(t *testing.T) {
	m := money.New(1, "USD")
	require.NoError(t, json.Unmarshal([]byte(`null`), &m))
	require.True(t, m.IsZeroValue())
}

func TestValue_CompositeLiteral(t *testing.T) {
	v, err := money.New(100000, "USD").Value()
	require.NoError(t, err)
	require.Equal(t, "(100000,USD)", v)

	v, err = money.Money{}.Value()
	require.NoError(t, err)
	require.Nil(t, v)
}

func TestScan_Composite(t *testing.T) {
	var m money.Money

	require.NoError(t, m.Scan("(100000,USD)"))
	require.Equal(t, int64(100000), m.Amount())
	require.Equal(t, "USD", m.Currency().Code)

	require.NoError(t, m.Scan([]byte("(-2500,UZS)")))
	require.Equal(t, int64(-2500), m.Amount())
	require.Equal(t, "UZS", m.Currency().Code)

	require.NoError(t, m.Scan(nil))
	require.True(t, m.IsZeroValue())

	require.Error(t, m.Scan("(100000)"))
	require.Error(t, m.Scan(42))
}

func TestValueScan_RoundTrip(t *testing.T) {
	orig := money.New(987654, "EUR")
	v, err := orig.Value()
	require.NoError(t, err)

	var got money.Money
	require.NoError(t, got.Scan(v))
	require.Equal(t, orig.Amount(), got.Amount())
	require.Equal(t, orig.Currency().Code, got.Currency().Code)
}
