package parser

import (
	"regexp"
	"strings"
)

// CardBalance is one card's reported balance from a balance-snapshot message.
type CardBalance struct {
	Bank      string
	CardLast4 string
	Amount    int64 // minor units
	Currency  string
}

// A balance snapshot lists one block per card:
//
//	🔹 HUMOCARD TBCBANK *8400
//	💵 6'924.46 UZS
var balanceBlockRe = regexp.MustCompile(
	emoji("🔹") + ws + `HUMOCARD` + ws + `(.*?)` + ws + `\*` + ws + `([0-9]+)` +
		nl + emoji("💵") + ws + `([0-9'.,]+)` + ws + `([A-Z]{3})`)

// ParseBalances extracts every card balance from a snapshot message. ok is true
// only when at least one block parsed; transaction notifications never match.
func ParseBalances(raw string) ([]CardBalance, bool) {
	matches := balanceBlockRe.FindAllStringSubmatch(raw, -1)
	if len(matches) == 0 {
		return nil, false
	}

	out := make([]CardBalance, 0, len(matches))
	for _, m := range matches {
		amount, err := parseDotMoney(m[3])
		if err != nil {
			continue
		}
		out = append(out, CardBalance{
			Bank:      strings.TrimSpace(m[1]),
			CardLast4: strings.TrimSpace(m[2]),
			Amount:    amount,
			Currency:  strings.TrimSpace(m[4]),
		})
	}
	if len(out) == 0 {
		return nil, false
	}

	return out, true
}
