package proxy

import (
	"bytes"
	"context"
	"encoding/json"
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
	url     string
	secret  string
	http    *http.Client
	timeout time.Duration
}

// New builds a Client. url is the full proxy endpoint. An empty url or secret
// leaves it disabled (Enabled reports false, Fetch errors).
func New(url, secret string, opts ...Option) *Client {
	c := &Client{
		url:     strings.TrimRight(url, "/"),
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
func (c *Client) Enabled() bool { return c.url != "" && c.secret != "" }

// Fetch posts qrURL to the proxy and returns the receipt HTML it scraped.
func (c *Client) Fetch(ctx context.Context, qrURL string) (string, error) {
	if !c.Enabled() {
		return "", errors.New("proxy: not configured")
	}

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	reqBody, err := json.Marshal(map[string]string{"url": qrURL})
	if err != nil {
		return "", fmt.Errorf("proxy marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("proxy request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.secret)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("proxy fetch: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("proxy read: %w", err)
	}

	var payload struct {
		HTML  string `json:"html"`
		Error string `json:"error"`
	}
	// Body is JSON on both success and error; tolerate a non-JSON body by
	// falling back to the raw text in the error path.
	_ = json.Unmarshal(body, &payload)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := payload.Error
		if msg == "" {
			msg = strings.TrimSpace(string(body))
		}

		return "", fmt.Errorf("proxy fetch: status %d: %s", resp.StatusCode, msg)
	}

	if payload.HTML == "" {
		return "", errors.New("proxy fetch: empty html")
	}

	return payload.HTML, nil
}
