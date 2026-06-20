package webpush

import (
	"time"

	wp "github.com/SherClockHolmes/webpush-go"
)

type Option func(*Sender)

// TTL sets how long (seconds) the push service retains an undelivered message.
// Non-positive values are ignored.
func TTL(seconds int) Option {
	return func(s *Sender) {
		if seconds > 0 {
			s.ttl = seconds
		}
	}
}

// Urgency sets the push priority header (very-low, low, normal, high). Empty is ignored.
func Urgency(u string) Option {
	return func(s *Sender) {
		if u != "" {
			s.urgency = wp.Urgency(u)
		}
	}
}

// Timeout bounds a single Send call. Non-positive values are ignored.
func Timeout(d time.Duration) Option {
	return func(s *Sender) {
		if d > 0 {
			s.timeout = d
		}
	}
}
