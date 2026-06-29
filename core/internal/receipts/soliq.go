package receipts

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"finance/pkg/proxy"
)

const (
	soliqPaymentURL = "https://new-ofd.soliq.uz/api/payment"
	// signSecret is soliq's payment signing key, baked into the public
	// ofd.soliq.uz web client.
	signSecret = "thisIsPaymentSecretKey123@#"
)

// fetchPayment signs and fetches a receipt's payment JSON via the proxy.
func (s *Service) fetchPayment(ctx context.Context, qrURL string) ([]byte, error) {
	p, ok := paymentParamsFromQR(qrURL)
	if !ok {
		return nil, ValidationError{"unrecognized receipt QR url"}
	}

	body, err := json.Marshal(map[string]string{
		"terminalId":  p.TerminalID,
		"paymentNo":   p.PaymentNo,
		"paymentDate": p.PaymentDate,
		"fiscalSign":  p.FiscalSign,
		"paymentType": "CHECK",
	})
	if err != nil {
		return nil, fmt.Errorf("marshal payment request: %w", err)
	}

	ts := strconv.FormatInt(time.Now().Unix(), 10)
	resp, err := s.proxy.Forward(ctx, proxy.Request{
		Method: http.MethodPost,
		URL:    soliqPaymentURL,
		Headers: map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
			"Origin":       "https://ofd.soliq.uz",
			"Referer":      "https://ofd.soliq.uz/",
			"X-Timestamp":  ts,
			"X-Signature":  paymentSignature(p.TerminalID, p.PaymentNo, ts),
		},
		Body: string(body),
	})
	if err != nil {
		return nil, err
	}
	if resp.Status < 200 || resp.Status >= 300 {
		return nil, fmt.Errorf("payment fetch: status %d: %s", resp.Status, resp.Body)
	}

	return []byte(resp.Body), nil
}

// paymentSignature is HMAC-SHA256(secret, "terminalId:paymentNo:timestamp") hex.
func paymentSignature(terminalID, paymentNo, ts string) string {
	mac := hmac.New(sha256.New, []byte(signSecret))
	mac.Write([]byte(terminalID + ":" + paymentNo + ":" + ts))

	return hex.EncodeToString(mac.Sum(nil))
}
