package categorysuggest

import (
	"reflect"
	"testing"
)

func TestParseNames(t *testing.T) {
	t.Parallel()

	candidates := []string{"Groceries", "Restaurants", "Utilities", "Transport"}

	tests := []struct {
		name string
		in   string
		want []string
	}{
		{
			name: "plain array, keeps order",
			in:   `["Restaurants","Groceries"]`,
			want: []string{"Restaurants", "Groceries"},
		},
		{
			name: "wrapped in prose and code fence",
			in:   "Sure:\n```json\n[\"Utilities\"]\n```\n",
			want: []string{"Utilities"},
		},
		{
			name: "case-insensitive match returns canonical spelling",
			in:   `["groceries","TRANSPORT"]`,
			want: []string{"Groceries", "Transport"},
		},
		{
			name: "drops names not in the allowed list",
			in:   `["Groceries","Vacation","Rent"]`,
			want: []string{"Groceries"},
		},
		{
			name: "dedupes",
			in:   `["Groceries","groceries"]`,
			want: []string{"Groceries"},
		},
		{
			name: "empty array",
			in:   `[]`,
			want: []string{},
		},
		{
			name: "no array",
			in:   "I cannot help.",
			want: nil,
		},
		{
			name: "malformed json",
			in:   `[Groceries, Restaurants]`,
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := parseNames(tt.in, candidates)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseNames(%q) = %#v, want %#v", tt.in, got, tt.want)
			}
		})
	}
}
