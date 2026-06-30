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
	mu       sync.Mutex
	posted   []pairing.Posting
	balances [][]parser.CardBalance
	failPerm bool
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

	f := &fakeFetcher{msgs: []telegram.Message{{ID: 100, Date: clock, Text: debit4853}}}

	if err := a.cycle(context.Background(), f); err != nil {
		t.Fatalf("cycle: %v", err)
	}
	if poster.count() != 1 {
		t.Fatalf("a parsed debit should post immediately, got %d", poster.count())
	}
	got := poster.posted[0]
	if got.Type != "expense" || got.FromCardLast4 != "4853" || got.ExternalID != "tg:1:100" {
		t.Fatalf("bad expense posting: %+v", got)
	}
	if st.State().Watermark != 100 {
		t.Errorf("watermark not advanced/persisted: %d", st.State().Watermark)
	}
}

// A card-to-card transfer now posts as two independent single legs; core pairs
// them server-side. lookout no longer emits a "transfer" posting itself.
func TestApp_TransferPostsBothLegsRaw(t *testing.T) {
	poster := &fakePoster{}
	a, _ := newApp(t, poster)
	f := &fakeFetcher{msgs: []telegram.Message{
		{ID: 200, Date: time.Now(), Text: xferDebit},
		{ID: 201, Date: time.Now(), Text: xferCredit},
	}}

	if err := a.cycle(context.Background(), f); err != nil {
		t.Fatalf("cycle: %v", err)
	}
	if poster.count() != 2 {
		t.Fatalf("expected both legs posted raw, got %d: %+v", poster.count(), poster.posted)
	}
	debit, credit := poster.posted[0], poster.posted[1]
	if debit.Type != "expense" || debit.FromCardLast4 != "4853" || debit.ExternalID != "tg:1:200" {
		t.Errorf("bad debit leg: %+v", debit)
	}
	if credit.Type != "income" || credit.ToCardLast4 != "8400" || credit.ExternalID != "tg:1:201" {
		t.Errorf("bad credit leg: %+v", credit)
	}
}

func TestApp_WatermarkPreventsReprocess(t *testing.T) {
	poster := &fakePoster{}
	a, _ := newApp(t, poster)
	clock := time.Date(2026, 6, 14, 10, 3, 0, 0, time.UTC)
	f := &fakeFetcher{msgs: []telegram.Message{{ID: 100, Date: clock, Text: debit4853}}}

	for i := 0; i < 5; i++ {
		if err := a.cycle(context.Background(), f); err != nil {
			t.Fatalf("cycle %d: %v", i, err)
		}
	}
	if poster.count() != 1 {
		t.Fatalf("message must post exactly once across repeated cycles, got %d", poster.count())
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
