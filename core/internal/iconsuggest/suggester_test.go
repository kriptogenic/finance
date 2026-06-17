package iconsuggest

import (
	"reflect"
	"testing"
)

func TestParseIcons(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want []string
	}{
		{
			name: "plain array",
			in:   `["shopping-cart","basket","meat"]`,
			want: []string{"shopping-cart", "basket", "meat"},
		},
		{
			name: "wrapped in prose and code fence",
			in:   "Here you go:\n```json\n[\"home\", \"home-2\"]\n```\nHope that helps!",
			want: []string{"home", "home-2"},
		},
		{
			name: "lowercases, trims, and dedupes",
			in:   `[" Wallet ", "wallet", "COINS"]`,
			want: []string{"wallet", "coins"},
		},
		{
			name: "drops malformed names",
			in:   `["ok-name", "Bad Name", "emoji_🍔", "has space", "fine2"]`,
			want: []string{"ok-name", "fine2"},
		},
		{
			name: "caps at maxIcons",
			in:   `["a","b","c","d","e","f","g","h","i","j"]`,
			want: []string{"a", "b", "c", "d", "e", "f", "g", "h"},
		},
		{
			name: "no array",
			in:   "I cannot help with that.",
			want: nil,
		},
		{
			name: "malformed json",
			in:   `[shopping-cart, basket]`,
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := parseIcons(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseIcons(%q) = %#v, want %#v", tt.in, got, tt.want)
			}
		})
	}
}
