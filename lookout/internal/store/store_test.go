package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStore_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")

	s, err := Open(path)
	if err != nil {
		t.Fatalf("open fresh: %v", err)
	}
	if got := s.State(); got.Watermark != 0 {
		t.Fatalf("fresh state should be empty, got %+v", got)
	}

	if err := s.Save(42); err != nil {
		t.Fatalf("save: %v", err)
	}

	s2, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if got := s2.State(); got.Watermark != 42 {
		t.Errorf("watermark: got %d want 42", got.Watermark)
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
