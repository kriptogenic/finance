// Package store persists the bot's durable state across restarts (§8): the
// last-processed message-ID watermark and the pending unpaired transfer legs.
// Persisting the legs means a restart never loses half a transfer; resuming from
// the watermark means history backfill re-fetches anything missed. It is a small
// JSON file written atomically (temp file + rename) so a crash mid-write can
// never corrupt it.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"finance/lookout/internal/pairing"
)

// State is the persisted snapshot. Watermark is the highest source message ID
// fully processed (delivered or buffered); Pending are transfer legs still
// waiting for a mate (§5.1, §8).
type State struct {
	Watermark int                  `json:"watermark"`
	Pending   []pairing.PendingLeg `json:"pending"`
}

// Store guards a State and its backing file. Save is safe for concurrent use;
// the watermark and buffer are otherwise mutated from the single poll goroutine.
type Store struct {
	path string
	mu   sync.Mutex
	st   State
}

// Open loads the state file, or starts from empty state if it does not yet
// exist. A malformed file is an error rather than silent data loss.
func Open(path string) (*Store, error) {
	s := &Store{path: path}
	data, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return s, nil // fresh start
	case err != nil:
		return nil, fmt.Errorf("read state %s: %w", path, err)
	}
	if err := json.Unmarshal(data, &s.st); err != nil {
		return nil, fmt.Errorf("parse state %s: %w", path, err)
	}
	return s, nil
}

// State returns a copy of the current persisted state.
func (s *Store) State() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snapshot()
}

// Save atomically persists a new watermark and pending-leg set. The watermark
// must only be advanced after a leg is either delivered (201/200) or safely
// buffered, so a crash re-processes rather than skips (§7, §8).
func (s *Store) Save(watermark int, pending []pairing.PendingLeg) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.st = State{Watermark: watermark, Pending: append([]pairing.PendingLeg(nil), pending...)}
	return s.flush()
}

func (s *Store) snapshot() State {
	return State{Watermark: s.st.Watermark, Pending: append([]pairing.PendingLeg(nil), s.st.Pending...)}
}

// flush writes the state to a temp file in the same directory and renames it
// over the target — an atomic replace on POSIX filesystems.
func (s *Store) flush() error {
	data, err := json.MarshalIndent(s.st, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("ensure state dir: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".state-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp state: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op once renamed

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp state: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync temp state: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp state: %w", err)
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		return fmt.Errorf("rename state into place: %w", err)
	}
	return nil
}
