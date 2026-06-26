package xlsx_test

import (
	"bytes"
	"strconv"
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

// A note/merchant value (untrusted: parsed from a bank SMS) that begins with a
// formula trigger must be stored as inert text — prefixed with a quote so a
// spreadsheet won't evaluate it. Safe values are left untouched.
func TestWorkbook_NeutralizesFormulaInjection(t *testing.T) {
	wb := xlsx.New(xlsx.WithSheet("T"))
	defer wb.Close()

	require.NoError(t, wb.Header("Note"))
	rows := []struct {
		in   string
		want string
	}{
		{`=HYPERLINK("http://evil","x")`, `'=HYPERLINK("http://evil","x")`},
		{`+1+1`, `'+1+1`},
		{`-2+3`, `'-2+3`},
		{`@SUM(A1:A9)`, `'@SUM(A1:A9)`},
		{"\t=1+1", "'\t=1+1"},
		{`Coffee Shop`, `Coffee Shop`}, // ordinary merchant, unchanged
	}
	for _, r := range rows {
		require.NoError(t, wb.AppendRow(r.in))
	}

	var buf bytes.Buffer
	_, err := wb.WriteTo(&buf)
	require.NoError(t, err)

	f, err := excelize.OpenReader(&buf)
	require.NoError(t, err)
	defer f.Close()

	for i, r := range rows {
		cell := "A" + strconv.Itoa(i+2) // +2: row 1 is the header
		got, gerr := f.GetCellValue("T", cell)
		require.NoError(t, gerr)
		require.Equal(t, r.want, got, "cell %s", cell)
	}
}
