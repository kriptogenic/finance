package proxy

import (
	"net/http"
	"time"
)

type Option func(*Client)

// Timeout bounds a single Fetch call. Non-positive values are ignored.
func Timeout(d time.Duration) Option {
	return func(c *Client) {
		if d > 0 {
			c.timeout = d
		}
	}
}

// HTTPClient overrides the underlying http.Client (e.g. for tests). Nil is ignored.
func HTTPClient(h *http.Client) Option {
	return func(c *Client) {
		if h != nil {
			c.http = h
		}
	}
}
