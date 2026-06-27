package receipts

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAmount(t *testing.T) {
	cases := map[string]int64{
		"16,480.00": 1648000,
		"0.00":      0,
		"8,240.00":  824000,
		"880.00":    88000,
		"":          0,
		"  12.50 ":  1250,
	}
	for in, want := range cases {
		got, err := parseAmount(in)
		require.NoError(t, err, in)
		assert.Equal(t, want, got, in)
	}

	_, err := parseAmount("abc")
	assert.Error(t, err)
}

func TestParseQRParams(t *testing.T) {
	terminal, seq, sign, at := ParseQRParams(
		"https://ofd.soliq.uz/check?t=TRM00099&r=4271&c=20240201134500&s=123456789012",
	)
	require.NotNil(t, terminal)
	assert.Equal(t, "TRM00099", *terminal)
	require.NotNil(t, seq)
	assert.Equal(t, 4271, *seq)
	require.NotNil(t, sign)
	assert.Equal(t, "123456789012", *sign)
	require.NotNil(t, at)
	assert.Equal(t, 2024, at.Year())
	assert.Equal(t, 13, at.Hour())
}

func TestParseHTML(t *testing.T) {
	raw, err := os.ReadFile("testdata/receipt.html")
	require.NoError(t, err)

	r, err := ParseHTML(string(raw))
	require.NoError(t, err)

	require.NotNil(t, r.ReceiptType)
	assert.Equal(t, "Savdo cheki/Sotuv", *r.ReceiptType)
	require.NotNil(t, r.MerchantName)
	assert.Equal(t, "OOO MAGAZIN", *r.MerchantName)
	require.NotNil(t, r.MerchantTIN)
	assert.Equal(t, "305123456", *r.MerchantTIN)
	require.NotNil(t, r.MerchantAddress)
	assert.Equal(t, "Toshkent sh., Chilonzor 12", *r.MerchantAddress)

	require.NotNil(t, r.ReceiptSeq)
	assert.Equal(t, 4271, *r.ReceiptSeq)
	require.NotNil(t, r.DeviceName)
	assert.Equal(t, "NKM-X", *r.DeviceName)
	require.NotNil(t, r.SerialNumber)
	assert.Equal(t, "SN9988", *r.SerialNumber)

	require.NotNil(t, r.ReceivedAt)
	assert.Equal(t, 2024, r.ReceivedAt.Year())

	assert.Equal(t, int64(0), r.PaidCash)
	assert.Equal(t, int64(1648000), r.PaidCard)
	require.NotNil(t, r.CardType)
	assert.Equal(t, "Shaxsiy", *r.CardType)
	assert.Equal(t, int64(1648000), r.TotalAmount)
	assert.Equal(t, int64(162800), r.TotalVAT)

	require.NotNil(t, r.MerchantLat)
	assert.Equal(t, "41.311081", *r.MerchantLat)
	require.NotNil(t, r.MerchantLng)
	assert.Equal(t, "69.240562", *r.MerchantLng)

	require.Len(t, r.Items, 2)

	non := r.Items[0]
	assert.Equal(t, "Non", non.Name)
	assert.Equal(t, "2", non.Quantity)
	assert.Equal(t, int64(824000), non.Price)
	assert.Equal(t, int64(88000), non.VATAmount)
	assert.Equal(t, 12, non.VATRate)
	require.NotNil(t, non.Barcode)
	assert.Equal(t, "4780000001", *non.Barcode)
	require.NotNil(t, non.IKPUCode)
	assert.Equal(t, "01234567", *non.IKPUCode)
	require.NotNil(t, non.IKPUName)
	assert.Equal(t, "Non mahsuloti", *non.IKPUName)
	require.NotNil(t, non.Unit)
	assert.Equal(t, "dona", *non.Unit)
	assert.Nil(t, non.MarkingCode)

	sut := r.Items[1]
	assert.Equal(t, "Sut", sut.Name)
	assert.Equal(t, int64(74800), sut.VATAmount)
	assert.Equal(t, 10, sut.VATRate)
	require.NotNil(t, sut.IKPUCode)
	assert.Equal(t, "07654321", *sut.IKPUCode)
}

// TestParseHTMLReal guards against the totals/card-type regression: a real
// ofd.soliq.uz receipt wraps the whole ticket in an outer table row and uses a
// backtick in "Jami to`lov:". Earlier substring matching read the document's
// last cell (the VAT total) into paid_cash/paid_card/card_type.
func TestParseHTMLReal(t *testing.T) {
	raw, err := os.ReadFile("testdata/receipt-2.html")
	require.NoError(t, err)

	r, err := ParseHTML(string(raw))
	require.NoError(t, err)

	require.NotNil(t, r.MerchantName)
	assert.Contains(t, *r.MerchantName, "ANGLESEY FOOD")
	require.NotNil(t, r.MerchantTIN)
	assert.Equal(t, "202099756", *r.MerchantTIN)

	// totals: the bug put 1,968.23 (VAT) into cash/card/card_type and 0 into total
	assert.Equal(t, int64(0), r.PaidCash)
	assert.Equal(t, int64(1837000), r.PaidCard)    // 18,370.00
	assert.Equal(t, int64(1837000), r.TotalAmount) // 18,370.00
	assert.Equal(t, int64(196823), r.TotalVAT)     // 1,968.23
	require.NotNil(t, r.CardType)
	assert.Equal(t, "Shaxsiy", *r.CardType)

	require.Len(t, r.Items, 4)
	first := r.Items[0]
	assert.Equal(t, "Logotipli paket Bio poelitilen 4 k gacha", first.Name)
	assert.Equal(t, "1", first.Quantity)
	assert.Equal(t, int64(40000), first.Price)    // 400.00
	assert.Equal(t, int64(4286), first.VATAmount) // 42.86
	assert.Equal(t, 12, first.VATRate)
	require.NotNil(t, first.Barcode)
	assert.Equal(t, "21005602", *first.Barcode)
}
