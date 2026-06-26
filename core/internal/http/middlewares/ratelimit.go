package middlewares

import (
	"math"
	"net/http"
	"time"

	"github.com/go-chi/httprate"
)

func AuthRateLimiter(attempts int, window time.Duration) func(http.Handler) http.Handler {
	rl := httprate.NewRateLimiter(attempts, window, httprate.WithKeyFuncs(httprate.KeyByRealIP))

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key, err := httprate.KeyByRealIP(r)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			if _, rate, serr := rl.Status(key); serr == nil && int(math.Round(rate)) >= attempts {
				http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
				return
			}

			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)

			if rec.status == http.StatusUnauthorized {
				_ = rl.Counter().Increment(key, time.Now().UTC().Truncate(window))
			}
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if !r.wroteHeader {
		r.status = code
		r.wroteHeader = true
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	return r.ResponseWriter.Write(b)
}
