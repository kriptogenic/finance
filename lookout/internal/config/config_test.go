package config

import "testing"

func TestValidateAPIURL(t *testing.T) {
	cases := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"https remote ok", "https://app.example.com/api", false},
		{"http loopback ok", "http://localhost:8080", false},
		{"http 127.0.0.1 ok", "http://127.0.0.1:8080", false},
		{"http remote rejected", "http://core:8080", true},
		{"http remote host rejected", "http://app.example.com/api", true},
		{"garbage rejected", "://nope", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateAPIURL(tc.url)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error for %q, got nil", tc.url)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.url, err)
			}
		})
	}
}
