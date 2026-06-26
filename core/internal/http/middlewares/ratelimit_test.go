package middlewares

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func handlerReturning(status int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
	})
}

func hit(t *testing.T, h http.Handler, ip string) int {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/accounts", nil)
	req.Header.Set("X-Real-IP", ip)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code
}

func TestAuthRateLimiter_ThrottlesAfterFailures(t *testing.T) {
	h := AuthRateLimiter(3, time.Minute)(handlerReturning(http.StatusUnauthorized))

	require.Equal(t, http.StatusUnauthorized, hit(t, h, "1.1.1.1"))
	require.Equal(t, http.StatusUnauthorized, hit(t, h, "1.1.1.1"))
	require.Equal(t, http.StatusUnauthorized, hit(t, h, "1.1.1.1"))
	require.Equal(t, http.StatusTooManyRequests, hit(t, h, "1.1.1.1"))
}

func TestAuthRateLimiter_SuccessNotCounted(t *testing.T) {
	h := AuthRateLimiter(2, time.Minute)(handlerReturning(http.StatusOK))

	for i := 0; i < 50; i++ {
		require.Equal(t, http.StatusOK, hit(t, h, "2.2.2.2"))
	}
}

func TestAuthRateLimiter_PerIP(t *testing.T) {
	h := AuthRateLimiter(2, time.Minute)(handlerReturning(http.StatusUnauthorized))

	require.Equal(t, http.StatusUnauthorized, hit(t, h, "3.3.3.3"))
	require.Equal(t, http.StatusUnauthorized, hit(t, h, "3.3.3.3"))
	require.Equal(t, http.StatusTooManyRequests, hit(t, h, "3.3.3.3"))

	// A different IP keeps its own budget.
	require.Equal(t, http.StatusUnauthorized, hit(t, h, "4.4.4.4"))
}

func TestAuthRateLimiter_OnlyCounts401(t *testing.T) {
	h := AuthRateLimiter(1, time.Minute)(handlerReturning(http.StatusBadRequest))

	for i := 0; i < 10; i++ {
		require.Equal(t, http.StatusBadRequest, hit(t, h, "5.5.5.5"))
	}
}
