package parser

import (
	"fmt"
	"strings"
)

// parseMoney converts a European/Uzbek-formatted amount into int64 minor units:
// '.' is the thousands separator and ',' the decimal separator, e.g.
// "697.945,26" → 69794526 (§4.2). It never uses float — money is integer minor
// units everywhere (§12). The decimal part is normalised to exactly two digits;
// a missing or differently-sized fraction is padded/rejected so we never
// silently lose or invent precision.
func parseMoney(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty amount")
	}

	intPart, fracPart, hasFrac := strings.Cut(s, ",")

	// Thousands separators are decorative; drop them from the integer part.
	intPart = strings.ReplaceAll(intPart, ".", "")
	if intPart == "" {
		intPart = "0"
	}

	frac := "00"
	if hasFrac {
		if len(fracPart) != 2 {
			return 0, fmt.Errorf("amount %q: fractional part must be 2 digits", s)
		}
		frac = fracPart
	}

	digits := intPart + frac
	var minor int64
	for _, r := range digits {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("amount %q: unexpected character %q", s, r)
		}
		minor = minor*10 + int64(r-'0')
	}

	return minor, nil
}
