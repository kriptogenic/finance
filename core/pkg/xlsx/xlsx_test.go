package xlsx_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"

	"finance/pkg/xlsx"
)

func TestWorkbook_RoundTrip(t *testing.T) {
	wb := xlsx.New(xlsx.WithSheet("Transactions"))
	defer wb.Close()

	require.NoError(t, wb.Header("Date", "Amount", "Currency", "Category", "Note"))
	require.NoError(t, wb.AppendRow("2026-06-26 10:00:00", -12.5, "USD", "Groceries", "milk"))

	var buf bytes.Buffer
	n, err := wb.WriteTo(&buf)
	require.NoError(t, err)
	require.Greater(t, n, int64(0))

	f, err := excelize.OpenReader(&buf)
	require.NoError(t, err)
	defer f.Close()

	rows, err := f.GetRows("Transactions")
	require.NoError(t, err)
	require.Equal(t, [][]string{
		{"Date", "Amount", "Currency", "Category", "Note"},
		{"2026-06-26 10:00:00", "-12.5", "USD", "Groceries", "milk"},
	}, rows)
}
