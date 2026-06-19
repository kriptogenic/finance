package delivery

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"

	"finance/lookout/generated/core"
	"finance/lookout/internal/pairing"
	"finance/lookout/internal/parser"
)

var ErrPermanent = errors.New("permanent ingest error")

type Config struct {
	MaxRetries  int
	BaseBackoff time.Duration
	MaxBackoff  time.Duration
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

type Client struct {
	api *core.ClientWithResponses
	cfg Config
	log *zap.Logger
}

func New(baseURL, token string, httpClient *http.Client, cfg Config, log *zap.Logger) (*Client, error) {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	opts := []core.ClientOption{core.WithHTTPClient(httpClient)}
	if token != "" {
		opts = append(opts, core.WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
			req.Header.Set("Authorization", "Bearer "+token)
			return nil
		}))
	}
	c, err := core.NewClientWithResponses(baseURL, opts...)
	if err != nil {
		return nil, fmt.Errorf("build ingest client: %w", err)
	}
	return &Client{api: c, cfg: cfg.withDefaults(), log: log}, nil
}

func (c *Client) Post(ctx context.Context, p pairing.Posting) error {
	body := toRequest(p)

	return c.retry(ctx, p.ExternalID, func() error { return c.attempt(ctx, body) })
}

// PostBalances delivers a card balance snapshot for reconciliation. The endpoint
// upserts per card, so re-delivery of the same snapshot is harmless.
func (c *Client) PostBalances(ctx context.Context, balances []parser.CardBalance, reportedAt time.Time) error {
	body := toBalanceRequest(balances, reportedAt)

	return c.retry(ctx, "balances", func() error { return c.attemptBalances(ctx, body) })
}

// retry runs do with exponential backoff until it succeeds, hits a permanent
// error, exhausts attempts, or the context is cancelled.
func (c *Client) retry(ctx context.Context, label string, do func() error) error {
	var backoff time.Duration
	for attempt := 0; ; attempt++ {
		err := do()
		if err == nil {
			return nil
		}

		if errors.Is(err, ErrPermanent) || ctx.Err() != nil {
			return err
		}
		if attempt >= c.cfg.MaxRetries {
			return fmt.Errorf("ingest %s: giving up after %d attempts: %w", label, attempt+1, err)
		}

		backoff = nextBackoff(backoff, c.cfg)
		c.log.Warn("ingest delivery failed, retrying",
			zap.String("target", label),
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

func (c *Client) attempt(ctx context.Context, body core.IngestTransactionRequest) error {
	resp, err := c.api.IngestTransactionWithResponse(ctx, body)
	if err != nil {
		return fmt.Errorf("post ingest: %w", err)
	}

	return classify(resp.StatusCode(), resp.Body)
}

func (c *Client) attemptBalances(ctx context.Context, body core.BalanceSnapshotRequest) error {
	resp, err := c.api.IngestBalancesWithResponse(ctx, body)
	if err != nil {
		return fmt.Errorf("post balances: %w", err)
	}

	return classify(resp.StatusCode(), resp.Body)
}

func classify(status int, body []byte) error {
	switch status {
	case http.StatusOK, http.StatusCreated, http.StatusNoContent:
		return nil
	case http.StatusBadRequest, http.StatusUnauthorized:
		return fmt.Errorf("%w: status %d: %s", ErrPermanent, status, string(body))
	default:
		return fmt.Errorf("unexpected ingest status %d: %s", status, string(body))
	}
}

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
