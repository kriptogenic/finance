package parser

import (
	"fmt"
	"strings"
)

// parseDotMoney parses balance-snapshot amounts, which use "'" (or spaces) as the
// thousands separator and "." as the decimal point, e.g. "6'924.46" -> 692446.
// This differs from parseMoney (transaction format: "." thousands, "," decimal).
func parseDotMoney(s string) (int64, error) {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "'", "")
	s = strings.ReplaceAll(s, " ", "")
	if s == "" {
		return 0, fmt.Errorf("empty amount")
	}

	intPart, fracPart, hasFrac := strings.Cut(s, ".")
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

	return digitsToMinor(intPart + frac)
}

func digitsToMinor(digits string) (int64, error) {
	var minor int64
	for _, r := range digits {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("digits %q: unexpected character %q", digits, r)
		}
		minor = minor*10 + int64(r-'0')
	}

	return minor, nil
}

func parseMoney(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty amount")
	}

	intPart, fracPart, hasFrac := strings.Cut(s, ",")

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

	return digitsToMinor(intPart + frac)
}
