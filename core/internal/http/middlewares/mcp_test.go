package middlewares

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMCPAuth(t *testing.T) {
	const token = "s3cret-token"

	okHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		name       string
		token      string
		authHeader string
		want       int
	}{
		{"valid", token, "Bearer " + token, http.StatusOK},
		{"missing header", token, "", http.StatusUnauthorized},
		{"wrong token", token, "Bearer nope", http.StatusUnauthorized},
		{"no bearer prefix", token, token, http.StatusUnauthorized},
		{"disabled empty token", "", "Bearer " + token, http.StatusUnauthorized},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := MCPAuth(tc.token)(okHandler)

			req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d", rec.Code, tc.want)
			}
		})
	}
}
