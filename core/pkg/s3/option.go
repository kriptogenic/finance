package s3

import "time"

type Option func(*Client)

// Timeout bounds a single Upload call. Non-positive values are ignored.
func Timeout(d time.Duration) Option {
	return func(c *Client) {
		if d > 0 {
			c.timeout = d
		}
	}
}
