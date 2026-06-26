package middlewares

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"finance/config"
	"finance/generated/api"
)

// handler builds the validator middleware over a next handler that records it
// was reached by writing 200, so tests can tell "passed auth" from "rejected".
func handler(t *testing.T, auth *config.Auth, ingest *config.Ingest) http.Handler {
	t.Helper()

	spec, err := api.GetSpec()
	require.NoError(t, err)
	spec.Servers = nil

	reached := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	return OpenAPIRequestValidator(spec, auth, ingest)(reached)
}

func basic(user, pass string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+pass))
}

func TestBasicAuth(t *testing.T) {
	auth := &config.Auth{Username: "admin", Password: "secret"}
	h := handler(t, auth, &config.Ingest{})

	cases := []struct {
		name       string
		authHeader string
		want       int
	}{
		{"no credentials", "", http.StatusUnauthorized},
		{"wrong password", basic("admin", "nope"), http.StatusUnauthorized},
		{"wrong user", basic("root", "secret"), http.StatusUnauthorized},
		{"valid credentials", basic("admin", "secret"), http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/accounts", nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			require.Equal(t, tc.want, rec.Code)
		})
	}
}

// The health probe opts out of auth (security: [] in the spec).
func TestHealthSkipsAuth(t *testing.T) {
	h := handler(t, &config.Auth{Username: "admin", Password: "secret"}, &config.Ingest{})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}

// The ingest endpoint keeps its own bearer scheme (lookout), independent of the
// global basic auth; security is validated before the body, so a bad token is
// rejected with 401 regardless of payload.
func TestIngestBearerAuth(t *testing.T) {
	h := handler(t, &config.Auth{Username: "admin", Password: "secret"}, &config.Ingest{Token: "tok"})

	t.Run("missing bearer", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/ingest/transactions", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("wrong bearer", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/ingest/transactions", nil)
		req.Header.Set("Authorization", "Bearer nope")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("basic creds rejected on ingest", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/ingest/transactions", nil)
		req.Header.Set("Authorization", basic("admin", "secret"))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

func TestIngestBearerAuth_EmptyTokenFailsClosed(t *testing.T) {
	h := handler(t, &config.Auth{Username: "admin", Password: "secret"}, &config.Ingest{})

	req := httptest.NewRequest(http.MethodPost, "/ingest/transactions", nil)
	req.Header.Set("Authorization", "Bearer anything")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}
