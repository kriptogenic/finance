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

// Client forwards HTTP requests through the UZ proxy, which performs them from
// an Uzbek IP (soliq.uz is geo-restricted) and returns the raw response.
type Client struct {
	url     string
	secret  string
	http    *http.Client
	timeout time.Duration
}

// New builds a Client. url is the full proxy endpoint. An empty url or secret
// leaves it disabled (Enabled reports false, Forward errors).
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

// Request is an HTTP request for the proxy to perform on our behalf.
type Request struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`
}

// Response is the proxy's relay of the upstream response.
type Response struct {
	Status int    `json:"status"`
	Body   string `json:"body"`
}

// Forward asks the proxy to perform fr and returns the upstream response.
func (c *Client) Forward(ctx context.Context, fr Request) (Response, error) {
	if !c.Enabled() {
		return Response{}, errors.New("proxy: not configured")
	}

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	reqBody, err := json.Marshal(fr)
	if err != nil {
		return Response{}, fmt.Errorf("proxy marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(reqBody))
	if err != nil {
		return Response{}, fmt.Errorf("proxy request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.secret)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return Response{}, fmt.Errorf("proxy forward: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Response{}, fmt.Errorf("proxy read: %w", err)
	}

	var payload struct {
		Status int    `json:"status"`
		Body   string `json:"body"`
		Error  string `json:"error"`
	}
	// Body is JSON on both success and error; tolerate a non-JSON body by
	// falling back to the raw text in the error path.
	_ = json.Unmarshal(body, &payload)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := payload.Error
		if msg == "" {
			msg = strings.TrimSpace(string(body))
		}

		return Response{}, fmt.Errorf("proxy forward: status %d: %s", resp.StatusCode, msg)
	}

	return Response{Status: payload.Status, Body: payload.Body}, nil
}
