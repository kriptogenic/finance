package audit

import (
	"testing"
	"time"
)

func TestParseDate(t *testing.T) {
	if _, ok, err := parseDate(""); err != nil || ok {
		t.Fatalf("empty: ok=%v err=%v", ok, err)
	}

	got, ok, err := parseDate("2026-02-15")
	if err != nil || !ok {
		t.Fatalf("valid: ok=%v err=%v", ok, err)
	}
	if !got.Equal(time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("parsed = %v", got)
	}

	if _, _, err := parseDate("15/02/2026"); err == nil {
		t.Fatal("expected error for bad format")
	}
}

func TestDateWindow_DefaultsAndBounds(t *testing.T) {
	from, to, err := dateWindow("", "")
	if err != nil {
		t.Fatal(err)
	}
	if !from.Equal(time.Unix(0, 0)) {
		t.Fatalf("default from = %v", from)
	}
	if to.Before(from) {
		t.Fatal("default to before from")
	}

	from, to, err = dateWindow("2026-01-01", "2026-01-31")
	if err != nil {
		t.Fatal(err)
	}
	if from.Month() != time.January || to.Day() != 31 {
		t.Fatalf("bounds from=%v to=%v", from, to)
	}
}

func TestForecastMonth(t *testing.T) {
	got := forecastMonth("2026-04")
	want := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("forecastMonth = %v, want %v", got, want)
	}

	// Empty / invalid fall back to the first of the current month.
	now := time.Now().UTC()
	got = forecastMonth("")
	if got.Year() != now.Year() || got.Month() != now.Month() || got.Day() != 1 {
		t.Fatalf("fallback = %v", got)
	}
}
