package anthropic

import "time"

type Option func(*Client)

// Model sets the Claude model id (e.g. "claude-haiku-4-5"). Empty is ignored.
func Model(model string) Option {
	return func(c *Client) {
		if model != "" {
			c.model = model
		}
	}
}

// MaxTokens caps the response length. Non-positive values are ignored.
func MaxTokens(n int64) Option {
	return func(c *Client) {
		if n > 0 {
			c.maxTokens = n
		}
	}
}

// Timeout bounds a single Complete call. Non-positive values are ignored.
func Timeout(d time.Duration) Option {
	return func(c *Client) {
		if d > 0 {
			c.timeout = d
		}
	}
}
