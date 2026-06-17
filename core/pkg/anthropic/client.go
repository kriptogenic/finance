// Package anthropic wraps the official Anthropic SDK behind a small,
// options-configured text-completion surface.
package anthropic

import (
	"context"
	"fmt"
	"strings"
	"time"

	sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

const (
	defaultModel     = "claude-haiku-4-5"
	defaultMaxTokens = 1024
	defaultTimeout   = 20 * time.Second
)

// Client is a thin wrapper around the Anthropic Messages API.
type Client struct {
	api       sdk.Client
	model     string
	maxTokens int64
	timeout   time.Duration
}

// New builds a Client for the given API key. Defaults can be overridden with
// Model, MaxTokens and Timeout options.
func New(apiKey string, opts ...Option) *Client {
	c := &Client{
		model:     defaultModel,
		maxTokens: defaultMaxTokens,
		timeout:   defaultTimeout,
	}
	for _, opt := range opts {
		opt(c)
	}
	c.api = sdk.NewClient(option.WithAPIKey(apiKey))

	return c
}

// Complete sends a single user prompt under the given system instruction and
// returns the concatenated text of the model's reply.
func (c *Client) Complete(ctx context.Context, system, prompt string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	msg, err := c.api.Messages.New(ctx, sdk.MessageNewParams{
		Model:     sdk.Model(c.model),
		MaxTokens: c.maxTokens,
		System:    []sdk.TextBlockParam{{Text: system}},
		Messages: []sdk.MessageParam{
			sdk.NewUserMessage(sdk.NewTextBlock(prompt)),
		},
	})
	if err != nil {
		return "", fmt.Errorf("anthropic complete: %w", err)
	}

	var b strings.Builder
	for _, block := range msg.Content {
		b.WriteString(block.Text)
	}

	return b.String(), nil
}
