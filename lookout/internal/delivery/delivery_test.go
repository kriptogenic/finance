package delivery

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap/zaptest"

	"finance/lookout/internal/pairing"
)

func testClient(t *testing.T, baseURL, token string) *Client {
	t.Helper()
	c, err := New(baseURL, token, nil, Config{MaxRetries: 4, BaseBackoff: time.Millisecond, MaxBackoff: 2 * time.Millisecond}, zaptest.NewLogger(t))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	return c
}

func TestToRequest_Expense(t *testing.T) {
	p := pairing.Posting{
		ExternalID:    "tg:1:100",
		Type:          "expense",
		FromCardLast4: "4853",
		Merchant:      "SP OOO HAVAS FOOD>T",
		Amount:        5755000,
		Date:          time.Date(2026, 6, 14, 10, 3, 0, 0, time.UTC),
		Tags:          []string{"humo"},
	}
	req := toRequest(p)
	if req.ExternalId != "tg:1:100" || string(req.Type) != "expense" || req.Amount != 5755000 {
		t.Fatalf("bad core fields: %+v", req)
	}
	if req.FromCardLast4 == nil || *req.FromCardLast4 != "4853" {
		t.Errorf("from_card_last4 not set")
	}
	if req.ToCardLast4 != nil {
		t.Errorf("expense must not set to_card_last4")
	}
	if req.Merchant == nil || *req.Merchant != "SP OOO HAVAS FOOD>T" {
		t.Errorf("merchant not forwarded")
	}
	if req.RateToBase != nil {
		t.Errorf("rate_to_base must be omitted for UZS base")
	}
}

func TestToRequest_Transfer(t *testing.T) {
	p := pairing.Posting{
		ExternalID:      "tg:transfer:10-11",
		Type:            "transfer",
		FromCardLast4:   "4853",
		ToCardLast4:     "8400",
		Amount:          100000000,
		TransferGroupID: "tg:transfer:10-11",
	}
	req := toRequest(p)
	if req.FromCardLast4 == nil || req.ToCardLast4 == nil {
		t.Fatalf("transfer must set both cards")
	}
	if req.Merchant != nil {
		t.Errorf("transfer must carry no merchant")
	}
	if req.TransferGroupId == nil || *req.TransferGroupId != "tg:transfer:10-11" {
		t.Errorf("transfer_group_id not set")
	}
}

func TestPost_CreatedAndAuth(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "x"})
	}))
	defer srv.Close()

	c := testClient(t, srv.URL, "secret123")
	if err := c.Post(context.Background(), pairing.Posting{ExternalID: "tg:1:1", Type: "expense", Amount: 1, FromCardLast4: "4853"}); err != nil {
		t.Fatalf("post: %v", err)
	}
	if gotAuth != "Bearer secret123" {
		t.Errorf("auth header: got %q", gotAuth)
	}
}

func TestPost_DedupedIsSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "x"})
	}))
	defer srv.Close()

	c := testClient(t, srv.URL, "")
	if err := c.Post(context.Background(), pairing.Posting{ExternalID: "tg:1:1", Type: "expense", Amount: 1}); err != nil {
		t.Fatalf("200 dedupe should be success, got %v", err)
	}
}

func TestPost_PermanentErrors(t *testing.T) {
	for _, code := range []int{http.StatusBadRequest, http.StatusUnauthorized} {
		var calls int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&calls, 1)
			w.WriteHeader(code)
			_, _ = io.WriteString(w, `{"message":"nope"}`)
		}))
		c := testClient(t, srv.URL, "")
		err := c.Post(context.Background(), pairing.Posting{ExternalID: "tg:1:1", Type: "expense", Amount: 1})
		if err == nil {
			t.Errorf("status %d should error", code)
		}
		if got := atomic.LoadInt32(&calls); got != 1 {
			t.Errorf("status %d should not be retried, got %d calls", code, got)
		}
		srv.Close()
	}
}

func TestPost_RetriesTransient(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) < 3 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "x"})
	}))
	defer srv.Close()

	c := testClient(t, srv.URL, "")
	if err := c.Post(context.Background(), pairing.Posting{ExternalID: "tg:1:1", Type: "expense", Amount: 1}); err != nil {
		t.Fatalf("should succeed after retries: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Errorf("expected 3 attempts, got %d", got)
	}
}
