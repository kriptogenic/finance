package receipts

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseMinor(t *testing.T) {
	cases := map[string]int64{
		"5990":   599000,
		"5990.0": 599000,
		"641.79": 64179,
		"641.7":  64170,
		"0":      0,
		"0.0":    0,
		"":       0,
		" 12.5 ": 1250,
		"-100":   -10000,
	}
	for in, want := range cases {
		assert.Equal(t, want, parseMinor(in), in)
	}
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

func TestPaymentParamsFromQR(t *testing.T) {
	p, ok := paymentParamsFromQR(
		"https://ofd.soliq.uz/?t=VG343420034337&r=34698&c=20260629165127&s=181435540172",
	)
	require.True(t, ok)
	assert.Equal(t, "VG343420034337", p.TerminalID)
	assert.Equal(t, "34698", p.PaymentNo)
	assert.Equal(t, "20260629165127", p.PaymentDate)
	assert.Equal(t, "181435540172", p.FiscalSign)

	_, ok = paymentParamsFromQR("https://ofd.soliq.uz/?t=VG343420034337&r=34698")
	assert.False(t, ok)
}

// TestPaymentSignature pins the HMAC scheme against a known request/signature
// captured from the live ofd.soliq.uz web client.
func TestPaymentSignature(t *testing.T) {
	got := paymentSignature("VG343420034337", "34698", "1782737660")
	assert.Equal(t, "520e6445ec4127fedc825eb8ba0fe5c3bab6d747778a68e716b2c9c898daacd7", got)
}

func TestParseJSON(t *testing.T) {
	raw, err := os.ReadFile("testdata/payment.json")
	require.NoError(t, err)

	r, err := ParseJSON(raw)
	require.NoError(t, err)

	require.NotNil(t, r.ReceiptType)
	assert.Equal(t, "Savdo cheki/Sotuv", *r.ReceiptType)
	require.NotNil(t, r.MerchantName)
	assert.Contains(t, *r.MerchantName, "ANGLESEY FOOD")
	require.NotNil(t, r.MerchantTIN)
	assert.Equal(t, "202099756", *r.MerchantTIN)
	require.NotNil(t, r.MerchantAddress)
	assert.Contains(t, *r.MerchantAddress, "Olmazor")

	require.NotNil(t, r.DeviceName)
	assert.Equal(t, "POS2k", *r.DeviceName)
	require.NotNil(t, r.SerialNumber)
	assert.Equal(t, "AFK-20251010-000337", *r.SerialNumber)
	require.NotNil(t, r.CardType)
	assert.Equal(t, "Shaxsiy", *r.CardType)
	require.NotNil(t, r.ReceiptSeq)
	assert.Equal(t, 34698, *r.ReceiptSeq)

	require.NotNil(t, r.ReceivedAt)
	assert.Equal(t, 2026, r.ReceivedAt.Year())
	assert.Equal(t, 16, r.ReceivedAt.Hour())
	// soliq timestamps are UTC+5; the stored instant must reflect that.
	_, offset := r.ReceivedAt.Zone()
	assert.Equal(t, 5*60*60, offset)
	assert.Equal(t, 11, r.ReceivedAt.UTC().Hour())

	assert.Equal(t, int64(0), r.PaidCash.Minor())
	assert.Equal(t, int64(599000), r.PaidCard.Minor())
	assert.Equal(t, int64(599000), r.TotalAmount.Minor())
	assert.Equal(t, int64(64179), r.TotalVAT.Minor())

	require.NotNil(t, r.MerchantLat)
	assert.Equal(t, "41.34714268633668", *r.MerchantLat)
	require.NotNil(t, r.MerchantLng)
	assert.Equal(t, "69.25738852794223", *r.MerchantLng)

	require.Len(t, r.Items, 1)
	it := r.Items[0]
	assert.Equal(t, "Ichimlik Coca Cola Zero t/b, 250ml", it.Name)
	assert.Equal(t, "1", it.Quantity)
	assert.Equal(t, int64(599000), it.Price.Minor())
	assert.Equal(t, int64(64179), it.VATAmount.Minor())
	assert.Equal(t, 12, it.VATRate)
	assert.Equal(t, int64(0), it.Discount.Minor())
	require.NotNil(t, it.Barcode)
	assert.Equal(t, "4780069000666", *it.Barcode)
	require.NotNil(t, it.IKPUCode)
	assert.Equal(t, "02202002001010003", *it.IKPUCode)
	require.NotNil(t, it.IKPUName)
	assert.Contains(t, *it.IKPUName, "COCA-COLA")
	require.NotNil(t, it.Unit)
	assert.Contains(t, *it.Unit, "дона")
	require.NotNil(t, it.MarkingCode)
	assert.Equal(t, "01047800690006662172AZ!+K8\"WA+,", *it.MarkingCode)
	assert.Nil(t, it.ConsignorTIN) // comitentTin 0 -> nil
}

func TestParseJSONError(t *testing.T) {
	_, err := ParseJSON([]byte(`{"data":null,"message":"Ma'lumot topilmadi","success":false}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Ma'lumot topilmadi")
}
