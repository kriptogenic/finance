package parser

import (
	"testing"
	"time"
)

func mustTashkent(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Tashkent")
	if err != nil {
		t.Fatalf("load Asia/Tashkent: %v", err)
	}
	return loc
}

// the real §4.1 fixtures, both single-line and one-field-per-line.
func TestParse_Fixtures(t *testing.T) {
	loc := mustTashkent(t)
	p := New(loc)

	cases := []struct {
		name      string
		raw       string
		dir       Direction
		amount    int64
		merchant  string
		cardType  string
		cardLast4 string
		balance   int64
		when      time.Time
	}{
		{
			name:      "single-line debit",
			raw:       "💸 Оплата➖ 57.550,00 UZS📍 SP OOO HAVAS FOOD>T💳 HUMOCARD *4853🕓 10:03 14.06.2026💰 697.945,26 UZS",
			dir:       Debit,
			amount:    5755000,
			merchant:  "SP OOO HAVAS FOOD>T",
			cardType:  "HUMOCARD",
			cardLast4: "4853",
			balance:   69794526,
			when:      time.Date(2026, 6, 14, 10, 3, 0, 0, loc),
		},
		{
			name:      "single-line credit",
			raw:       "🎉 Пополнение➕ 520.000,00 UZS📍 DAVR MOBILE P2P U2H>💳 HUMOCARD *4853🕓 23:17 13.06.2026💰 2.088.245,26 UZS",
			dir:       Credit,
			amount:    52000000,
			merchant:  "DAVR MOBILE P2P U2H>",
			cardType:  "HUMOCARD",
			cardLast4: "4853",
			balance:   208824526,
			when:      time.Date(2026, 6, 13, 23, 17, 0, 0, loc),
		},
		{
			name:      "single-line transfer debit leg",
			raw:       "💸 Операция➖ 1.000.000,00 UZS📍 TBC HUMO P2P>TASHKEN💳 HUMOCARD *4853🕓 09:36 14.06.2026💰 1.088.245,26 UZS",
			dir:       Debit,
			amount:    100000000,
			merchant:  "TBC HUMO P2P>TASHKEN",
			cardType:  "HUMOCARD",
			cardLast4: "4853",
			balance:   108824526,
			when:      time.Date(2026, 6, 14, 9, 36, 0, 0, loc),
		},
		{
			name:      "single-line transfer credit leg",
			raw:       "🎉 Пополнение➕ 1.000.000,00 UZS📍 TBC HUMO P2P>TASHKEN💳 HUMOCARD *8400🕓 09:36 14.06.2026💰 1.110.241,56 UZS",
			dir:       Credit,
			amount:    100000000,
			merchant:  "TBC HUMO P2P>TASHKEN",
			cardType:  "HUMOCARD",
			cardLast4: "8400",
			balance:   111024156,
			when:      time.Date(2026, 6, 14, 9, 36, 0, 0, loc),
		},
		{
			name:      "multi-line debit (P2P to person)",
			raw:       "💸 Оплата\n➖ 500.000,00 UZS\n📍 TBC P2P S HUMO NA UZ\n💳 HUMOCARD *8400\n🕓 09:39 14.06.2026\n💰 610.241,56 UZS",
			dir:       Debit,
			amount:    50000000,
			merchant:  "TBC P2P S HUMO NA UZ",
			cardType:  "HUMOCARD",
			cardLast4: "8400",
			balance:   61024156,
			when:      time.Date(2026, 6, 14, 9, 39, 0, 0, loc),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := p.Parse(1, 100, tc.raw)
			if !got.Parsed {
				t.Fatalf("expected Parsed=true, got false for %q", tc.raw)
			}
			if got.Direction != tc.dir {
				t.Errorf("direction: got %v want %v", got.Direction, tc.dir)
			}
			if got.Amount != tc.amount {
				t.Errorf("amount: got %d want %d", got.Amount, tc.amount)
			}
			if got.Merchant != tc.merchant {
				t.Errorf("merchant: got %q want %q", got.Merchant, tc.merchant)
			}
			if got.CardType != tc.cardType {
				t.Errorf("cardType: got %q want %q", got.CardType, tc.cardType)
			}
			if got.CardLast4 != tc.cardLast4 {
				t.Errorf("cardLast4: got %q want %q", got.CardLast4, tc.cardLast4)
			}
			if got.BalanceAfter != tc.balance {
				t.Errorf("balance: got %d want %d", got.BalanceAfter, tc.balance)
			}
			if !got.Time.Equal(tc.when) {
				t.Errorf("time: got %s want %s", got.Time, tc.when)
			}
			if got.RawText != tc.raw {
				t.Errorf("raw text not retained")
			}
		})
	}
}

// Direction must come from the sign marker, not the descriptive word: Операция
// appears on both a debit and (hypothetically) other legs, so only ➖/➕ decides.
func TestParse_DirectionFromSignNotWord(t *testing.T) {
	p := New(mustTashkent(t))
	rec := p.Parse(1, 1, "💸 Операция➖ 1.000.000,00 UZS📍 X>Y💳 HUMOCARD *4853🕓 09:36 14.06.2026💰 1.088.245,26 UZS")
	if rec.Direction != Debit {
		t.Fatalf("Операция with ➖ must be Debit, got %v", rec.Direction)
	}
	if rec.TypeWord != "Операция" {
		t.Errorf("type word: got %q want Операция", rec.TypeWord)
	}
}

// An optional U+FE0F variation selector after a marker emoji must still parse.
func TestParse_VariationSelectorTolerant(t *testing.T) {
	p := New(mustTashkent(t))
	raw := "💸 Оплата➖ 57.550,00 UZS📍️ SP OOO HAVAS FOOD>T💳️ HUMOCARD *4853🕓️ 10:03 14.06.2026💰️ 697.945,26 UZS"
	rec := p.Parse(1, 1, raw)
	if !rec.Parsed {
		t.Fatalf("variation-selector message should still parse")
	}
	if rec.Amount != 5755000 {
		t.Errorf("amount: got %d want 5755000", rec.Amount)
	}
}

// Fail-loud: an unparseable message must not panic and must come back
// Parsed=false with the raw text retained, never dropped (§4.2).
func TestParse_FailLoudRawPassthrough(t *testing.T) {
	p := New(mustTashkent(t))
	raw := "this is not a bank notification at all"
	rec := p.Parse(7, 42, raw)
	if rec.Parsed {
		t.Fatalf("garbage should not parse")
	}
	if rec.RawText != raw {
		t.Errorf("raw text not retained on failure")
	}
	if rec.ChatID != 7 || rec.MessageID != 42 {
		t.Errorf("ids not retained on failure: %d/%d", rec.ChatID, rec.MessageID)
	}
	if rec.ExternalID() != "tg:7:42" {
		t.Errorf("external id: got %q", rec.ExternalID())
	}
}

func TestParseMoney(t *testing.T) {
	cases := map[string]int64{
		"697.945,26":   69794526,
		"57.550,00":    5755000,
		"1.000.000,00": 100000000,
		"520.000,00":   52000000,
		"0,01":         1,
		"5":            500,
		"1.234":        123400,
	}
	for in, want := range cases {
		got, err := parseMoney(in)
		if err != nil {
			t.Errorf("parseMoney(%q): unexpected error %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("parseMoney(%q): got %d want %d", in, got, want)
		}
	}

	bad := []string{"", "1,5", "1,555", "12,3x", "abc"}
	for _, in := range bad {
		if _, err := parseMoney(in); err == nil {
			t.Errorf("parseMoney(%q): expected error, got nil", in)
		}
	}
}
