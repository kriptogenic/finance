package middlewares

import "net/http"

// MCPAuth guards the /mcp endpoint with a bearer token. An empty token disables
// the endpoint: authenticateBearer rejects every request when token == "".
func MCPAuth(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := authenticateBearer(r, token); err != nil {
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
