package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"finance/lookout/internal/pairing"
	"finance/lookout/internal/parser"
)

func TestStore_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")

	s, err := Open(path)
	if err != nil {
		t.Fatalf("open fresh: %v", err)
	}
	if got := s.State(); got.Watermark != 0 || len(got.Pending) != 0 {
		t.Fatalf("fresh state should be empty, got %+v", got)
	}

	legs := []pairing.PendingLeg{{
		Record:    parser.Record{ChatID: 1, MessageID: 42, Direction: parser.Debit, Amount: 100000000, CardLast4: "4853", Time: time.Now().UTC().Truncate(time.Second), Parsed: true},
		ArrivalAt: time.Now().UTC().Truncate(time.Second),
	}}
	if err := s.Save(42, legs); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Reopen from disk and verify the watermark and pending legs survived.
	s2, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	got := s2.State()
	if got.Watermark != 42 {
		t.Errorf("watermark: got %d want 42", got.Watermark)
	}
	if len(got.Pending) != 1 || got.Pending[0].Record.MessageID != 42 || got.Pending[0].Record.CardLast4 != "4853" {
		t.Fatalf("pending legs not restored: %+v", got.Pending)
	}
}

func TestStore_MalformedFileIsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(path); err == nil {
		t.Fatalf("malformed state should error, not silently reset")
	}
}
