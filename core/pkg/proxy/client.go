package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultTimeout = 30 * time.Second

// Client calls the UZ proxy's /fetch endpoint.
type Client struct {
	baseURL string
	secret  string
	http    *http.Client
	timeout time.Duration
}

// New builds a Client. An empty baseURL or secret leaves it disabled (Enabled
// reports false, Fetch errors).
func New(baseURL, secret string, opts ...Option) *Client {
	c := &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		secret:  secret,
		http:    http.DefaultClient,
		timeout: defaultTimeout,
	}
	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Enabled reports whether the proxy is configured.
func (c *Client) Enabled() bool { return c.baseURL != "" && c.secret != "" }

// Fetch posts qrURL to the proxy and returns the raw receipt HTML.
func (c *Client) Fetch(ctx context.Context, qrURL string) (string, error) {
	if !c.Enabled() {
		return "", errors.New("uzproxy: not configured")
	}

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/fetch", strings.NewReader(qrURL))
	if err != nil {
		return "", fmt.Errorf("uzproxy request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.secret)
	req.Header.Set("Content-Type", "text/plain")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("uzproxy fetch: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("uzproxy read: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("uzproxy fetch: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return string(body), nil
}
