// Package delivery posts Postings to the finance app's ingest endpoint. It wraps
// the client generated from specs/api.yaml (so the contract can't drift, §14.1)
// with bearer auth, the Posting→IngestTransactionRequest mapping (§5), and
// generous exponential-backoff retries (§7). It treats 201 (created) and 200
// (already ingested / deduped) as success so the caller may advance its
// watermark; 400/401 are permanent and not retried (§7, §14.2).
package delivery

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"

	"finance/lookout/generated/api"
	"finance/lookout/internal/pairing"
)

// ErrPermanent marks a response the bot must not retry and must not treat as
// delivered: a 400 (unknown card / invalid shape) or 401 (bad token). The
// operator must intervene; the watermark must not advance past it (§7).
var ErrPermanent = errors.New("permanent ingest error")

// Config tunes the retry behaviour. Zero values fall back to sane defaults.
type Config struct {
	MaxRetries  int           // attempts beyond the first (default 6)
	BaseBackoff time.Duration // first backoff step (default 1s)
	MaxBackoff  time.Duration // backoff ceiling (default 60s)
}

func (c Config) withDefaults() Config {
	if c.MaxRetries <= 0 {
		c.MaxRetries = 6
	}
	if c.BaseBackoff <= 0 {
		c.BaseBackoff = time.Second
	}
	if c.MaxBackoff <= 0 {
		c.MaxBackoff = 60 * time.Second
	}
	return c
}

// Client posts Postings to the ingest endpoint.
type Client struct {
	api *api.ClientWithResponses
	cfg Config
	log *zap.Logger
}

// New builds a delivery Client targeting baseURL (the app's FINANCE_API_URL). A
// non-empty token is sent as a bearer on every request; an empty token matches
// the app's open-auth local mode (§7). httpClient may be nil (defaults applied).
func New(baseURL, token string, httpClient *http.Client, cfg Config, log *zap.Logger) (*Client, error) {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	opts := []api.ClientOption{api.WithHTTPClient(httpClient)}
	if token != "" {
		opts = append(opts, api.WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
			req.Header.Set("Authorization", "Bearer "+token)
			return nil
		}))
	}
	c, err := api.NewClientWithResponses(baseURL, opts...)
	if err != nil {
		return nil, fmt.Errorf("build ingest client: %w", err)
	}
	return &Client{api: c, cfg: cfg.withDefaults(), log: log}, nil
}

// Post delivers one Posting, retrying transient failures (network errors, 5xx)
// with exponential backoff until success, a permanent error, or ctx ends. On
// success (201/200) it returns nil and the caller may advance the watermark.
func (c *Client) Post(ctx context.Context, p pairing.Posting) error {
	body := toRequest(p)

	var backoff time.Duration
	for attempt := 0; ; attempt++ {
		err := c.attempt(ctx, body)
		if err == nil {
			return nil
		}
		// Permanent errors and a cancelled context are not retryable.
		if errors.Is(err, ErrPermanent) || ctx.Err() != nil {
			return err
		}
		if attempt >= c.cfg.MaxRetries {
			return fmt.Errorf("ingest %s: giving up after %d attempts: %w", p.ExternalID, attempt+1, err)
		}

		backoff = nextBackoff(backoff, c.cfg)
		c.log.Warn("ingest delivery failed, retrying",
			zap.String("external_id", p.ExternalID),
			zap.Int("attempt", attempt+1),
			zap.Duration("backoff", backoff),
			zap.Error(err),
		)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
	}
}

// attempt performs a single POST and classifies the result.
func (c *Client) attempt(ctx context.Context, body api.IngestTransactionRequest) error {
	resp, err := c.api.IngestTransactionWithResponse(ctx, body)
	if err != nil {
		return fmt.Errorf("post ingest: %w", err) // transport error → retryable
	}
	switch resp.StatusCode() {
	case http.StatusCreated, http.StatusOK: // 201 created / 200 deduped → success
		return nil
	case http.StatusBadRequest, http.StatusUnauthorized:
		return fmt.Errorf("%w: status %d: %s", ErrPermanent, resp.StatusCode(), string(resp.Body))
	default:
		return fmt.Errorf("unexpected ingest status %d: %s", resp.StatusCode(), string(resp.Body))
	}
}

// nextBackoff doubles the previous backoff (starting at BaseBackoff), capped at
// MaxBackoff.
func nextBackoff(prev time.Duration, cfg Config) time.Duration {
	if prev <= 0 {
		return cfg.BaseBackoff
	}
	next := prev * 2
	if next > cfg.MaxBackoff {
		return cfg.MaxBackoff
	}
	return next
}
