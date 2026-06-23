package app

import (
	"context"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"

	"finance/lookout/internal/delivery"
	"finance/lookout/internal/pairing"
	"finance/lookout/internal/parser"
	"finance/lookout/internal/recon"
	"finance/lookout/internal/store"
	"finance/lookout/internal/telegram"
)

type fakeFetcher struct {
	msgs []telegram.Message
}

func (f *fakeFetcher) ChatID() int64 { return 1 }

func (f *fakeFetcher) FetchNewer(_ context.Context, sinceID int) ([]telegram.Message, error) {
	var out []telegram.Message
	for _, m := range f.msgs {
		if m.ID > sinceID {
			out = append(out, m)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

type fakePoster struct {
	mu        sync.Mutex
	posted    []pairing.Posting
	balances  [][]parser.CardBalance
	failPerm  bool
	failTimes int
}

func (p *fakePoster) Post(_ context.Context, post pairing.Posting) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.failPerm {
		return delivery.ErrPermanent
	}
	p.posted = append(p.posted, post)
	return nil
}

func (p *fakePoster) PostBalances(_ context.Context, balances []parser.CardBalance, _ time.Time) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.failPerm {
		return delivery.ErrPermanent
	}
	p.balances = append(p.balances, balances)
	return nil
}

func (p *fakePoster) count() int { p.mu.Lock(); defer p.mu.Unlock(); return len(p.posted) }

func (p *fakePoster) balanceBatches() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.balances)
}

func newApp(t *testing.T, poster *fakePoster) (*App, *store.Store) {
	t.Helper()
	loc, _ := time.LoadLocation("Asia/Tashkent")
	st, err := store.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	a := New(
		parser.New(loc),
		pairing.New(2*time.Minute, 5*time.Minute),
		poster,
		poster,
		st,
		recon.New(zap.NewNop()),
		time.Minute,
		zap.NewNop(),
	)
	return a, st
}

const (
	debit4853  = "💸 Оплата\n➖ 57.550,00 UZS\n📍 SP OOO HAVAS FOOD>T\n💳 HUMOCARD *4853\n🕓 10:03 14.06.2026\n💰 697.945,26 UZS"
	xferDebit  = "💸 Операция\n➖ 1.000.000,00 UZS\n📍 TBC HUMO P2P>TASHKEN\n💳 HUMOCARD *4853\n🕓 09:36 14.06.2026\n💰 1.088.245,26 UZS"
	xferCredit = "🎉 Пополнение\n➕ 1.000.000,00 UZS\n📍 TBC HUMO P2P>TASHKEN\n💳 HUMOCARD *8400\n🕓 09:36 14.06.2026\n💰 1.110.241,56 UZS"
)

func TestApp_ExpenseFlow(t *testing.T) {
	poster := &fakePoster{}
	a, st := newApp(t, poster)
	clock := time.Date(2026, 6, 14, 10, 3, 0, 0, time.UTC)
	a.now = func() time.Time { return clock }

	f := &fakeFetcher{msgs: []telegram.Message{{ID: 100, Date: clock, Text: debit4853}}}

	if err := a.cycle(context.Background(), f); err != nil {
		t.Fatalf("cycle: %v", err)
	}
	if poster.count() != 0 {
		t.Fatalf("lone debit should be held, not posted immediately; got %d", poster.count())
	}
	if st.State().Watermark != 100 {
		t.Errorf("watermark not advanced/persisted: %d", st.State().Watermark)
	}

	clock = clock.Add(6 * time.Minute)
	if err := a.cycle(context.Background(), f); err != nil {
		t.Fatalf("cycle (flush): %v", err)
	}
	if poster.count() != 1 {
		t.Fatalf("expected 1 expense after hold, got %d", poster.count())
	}
	got := poster.posted[0]
	if got.Type != "expense" || got.FromCardLast4 != "4853" || got.ExternalID != "tg:1:100" {
		t.Fatalf("bad expense posting: %+v", got)
	}
}

func TestApp_TransferPairing(t *testing.T) {
	poster := &fakePoster{}
	a, _ := newApp(t, poster)
	f := &fakeFetcher{msgs: []telegram.Message{
		{ID: 200, Date: time.Now(), Text: xferDebit},
		{ID: 201, Date: time.Now(), Text: xferCredit},
	}}

	if err := a.cycle(context.Background(), f); err != nil {
		t.Fatalf("cycle: %v", err)
	}
	if poster.count() != 1 {
		t.Fatalf("expected exactly 1 transfer posting, got %d: %+v", poster.count(), poster.posted)
	}
	got := poster.posted[0]
	if got.Type != "transfer" || got.FromCardLast4 != "4853" || got.ToCardLast4 != "8400" {
		t.Fatalf("bad transfer: %+v", got)
	}
	if got.ExternalID != "tg:transfer:200-201" {
		t.Errorf("transfer external id: %q", got.ExternalID)
	}
}

func TestApp_WatermarkPreventsReprocess(t *testing.T) {
	poster := &fakePoster{}
	a, _ := newApp(t, poster)
	clock := time.Date(2026, 6, 14, 10, 3, 0, 0, time.UTC)
	a.now = func() time.Time { return clock }
	f := &fakeFetcher{msgs: []telegram.Message{{ID: 100, Date: clock, Text: debit4853}}}

	for i := 0; i < 5; i++ {
		if err := a.cycle(context.Background(), f); err != nil {
			t.Fatalf("cycle %d: %v", i, err)
		}
		clock = clock.Add(3 * time.Minute)
	}
	if poster.count() != 1 {
		t.Fatalf("message must post exactly once across repeated cycles, got %d", poster.count())
	}
}

func TestApp_RestartResumesPendingLeg(t *testing.T) {
	poster := &fakePoster{}
	a, st := newApp(t, poster)

	f1 := &fakeFetcher{msgs: []telegram.Message{{ID: 200, Date: time.Now(), Text: xferDebit}}}
	if err := a.cycle(context.Background(), f1); err != nil {
		t.Fatal(err)
	}
	if poster.count() != 0 {
		t.Fatalf("lone leg should not post yet, got %d", poster.count())
	}
	if len(st.State().Pending) != 1 {
		t.Fatalf("pending leg should be persisted, have %d", len(st.State().Pending))
	}

	loc, _ := time.LoadLocation("Asia/Tashkent")
	a2 := New(parser.New(loc), pairing.New(2*time.Minute, 5*time.Minute), poster, poster, st, recon.New(zap.NewNop()), time.Minute, zap.NewNop())

	f2 := &fakeFetcher{msgs: []telegram.Message{
		{ID: 200, Date: time.Now(), Text: xferDebit},
		{ID: 201, Date: time.Now(), Text: xferCredit},
	}}
	if err := a2.cycle(context.Background(), f2); err != nil {
		t.Fatal(err)
	}
	if poster.count() != 1 || poster.posted[0].Type != "transfer" {
		t.Fatalf("restored leg should pair into a transfer, got %d: %+v", poster.count(), poster.posted)
	}
}

func TestApp_UnparsableAdvancesWithoutPosting(t *testing.T) {
	poster := &fakePoster{}
	a, st := newApp(t, poster)
	f := &fakeFetcher{msgs: []telegram.Message{{ID: 100, Date: time.Now(), Text: "not a notification"}}}

	if err := a.cycle(context.Background(), f); err != nil {
		t.Fatal(err)
	}
	if poster.count() != 0 {
		t.Fatalf("unparsed message must not post")
	}
	if st.State().Watermark != 100 {
		t.Errorf("watermark should advance past unparsed message")
	}
}

func TestApp_BalanceSnapshotRouted(t *testing.T) {
	poster := &fakePoster{}
	a, st := newApp(t, poster)
	snapshot := "🔹 HUMOCARD TBCBANK *8400\n💵 6'924.46 UZS\n\n🔹 HUMOCARD IPAKYULIBANK *4853\n💵 69.86 UZS"
	f := &fakeFetcher{msgs: []telegram.Message{{ID: 300, Date: time.Now(), Text: snapshot}}}

	if err := a.cycle(context.Background(), f); err != nil {
		t.Fatalf("cycle: %v", err)
	}
	if poster.count() != 0 {
		t.Fatalf("snapshot must not create transactions, got %d", poster.count())
	}
	if poster.balanceBatches() != 1 {
		t.Fatalf("expected 1 balance batch delivered, got %d", poster.balanceBatches())
	}
	if got := len(poster.balances[0]); got != 2 {
		t.Fatalf("expected 2 card balances, got %d", got)
	}
	if st.State().Watermark != 300 {
		t.Errorf("watermark should advance past snapshot, got %d", st.State().Watermark)
	}
}

func TestApp_TransactionReportsBalance(t *testing.T) {
	poster := &fakePoster{}
	a, _ := newApp(t, poster)
	clock := time.Date(2026, 6, 14, 10, 3, 0, 0, time.UTC)
	a.now = func() time.Time { return clock }

	f := &fakeFetcher{msgs: []telegram.Message{{ID: 100, Date: clock, Text: debit4853}}}
	if err := a.cycle(context.Background(), f); err != nil {
		t.Fatalf("cycle: %v", err)
	}

	if poster.balanceBatches() != 1 {
		t.Fatalf("transaction should report its card balance, got %d batches", poster.balanceBatches())
	}
	bal := poster.balances[0]
	if len(bal) != 1 || bal[0].CardLast4 != "4853" || bal[0].Amount != 69794526 || bal[0].Currency != "UZS" {
		t.Fatalf("bad reported balance: %+v", bal)
	}
}

func TestApp_PermanentErrorHoldsWatermark(t *testing.T) {
	poster := &fakePoster{failPerm: true}
	a, st := newApp(t, poster)

	f := &fakeFetcher{msgs: []telegram.Message{
		{ID: 200, Date: time.Now(), Text: xferDebit},
		{ID: 201, Date: time.Now(), Text: xferCredit},
	}}

	err := a.cycle(context.Background(), f)
	if err == nil {
		t.Fatal("expected the cycle to surface the permanent error")
	}

	if wm := st.State().Watermark; wm >= 201 {
		t.Errorf("watermark must not advance past a failed delivery, got %d", wm)
	}
}
