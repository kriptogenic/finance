package parser

import "testing"

func TestParseBalances_Fixture(t *testing.T) {
	raw := `🔹 HUMOCARD TBCBANK *8400
💵 6'924.46 UZS

🔹 HUMOCARD TBCBANK *7351
💵 960.20 UZS

🔹 HUMOCARD KAPITALBANK *2953
💵 0.00 UZS

🔹 HUMOCARD AO ANOR BANK *4234
💵 0.00 UZS

🔹 HUMOCARD IPAKYULIBANK *4853
💵 69.86 UZS`

	got, ok := ParseBalances(raw)
	if !ok {
		t.Fatalf("ParseBalances ok=false, want true")
	}

	want := []CardBalance{
		{Bank: "TBCBANK", CardLast4: "8400", Amount: 692446, Currency: "UZS"},
		{Bank: "TBCBANK", CardLast4: "7351", Amount: 96020, Currency: "UZS"},
		{Bank: "KAPITALBANK", CardLast4: "2953", Amount: 0, Currency: "UZS"},
		{Bank: "AO ANOR BANK", CardLast4: "4234", Amount: 0, Currency: "UZS"},
		{Bank: "IPAKYULIBANK", CardLast4: "4853", Amount: 6986, Currency: "UZS"},
	}

	if len(got) != len(want) {
		t.Fatalf("got %d cards, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("card %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// A transaction notification must not be mistaken for a balance snapshot.
func TestParseBalances_IgnoresTransaction(t *testing.T) {
	raw := `💸 Оплата
➖ 57.550,00 UZS
📍 SP OOO HAVAS FOOD>T
💳 HUMOCARD *4853
🕓 10:03 14.06.2026
💰 697.945,26 UZS`

	if _, ok := ParseBalances(raw); ok {
		t.Fatalf("ParseBalances matched a transaction message, want no match")
	}
}
