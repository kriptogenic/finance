// Package webpush wraps SherClockHolmes/webpush-go behind a small,
// options-configured sender for delivering encrypted Web Push payloads.
package webpush

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	wp "github.com/SherClockHolmes/webpush-go"
)

// ErrGone signals that the push service rejected the subscription as expired
// (HTTP 404/410). Callers should delete the stored subscription.
var ErrGone = errors.New("subscription gone")

const (
	defaultTTL     = 86400 // seconds the push service retains an undelivered message
	defaultTimeout = 10 * time.Second
)

// Subscription is a browser PushSubscription: its endpoint plus the two keys
// used to encrypt the payload.
type Subscription struct {
	Endpoint string
	P256dh   string
	Auth     string
}

// Sender delivers payloads to push endpoints, signed with a VAPID keypair.
type Sender struct {
	publicKey  string
	privateKey string
	subscriber string
	ttl        int
	urgency    wp.Urgency
	timeout    time.Duration
}

// New builds a Sender for the given VAPID keypair and subscriber contact
// (a mailto: or https URL). Defaults can be overridden with options.
func New(publicKey, privateKey, subscriber string, opts ...Option) *Sender {
	s := &Sender{
		publicKey:  publicKey,
		privateKey: privateKey,
		subscriber: subscriber,
		ttl:        defaultTTL,
		urgency:    wp.UrgencyNormal,
		timeout:    defaultTimeout,
	}
	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Send delivers payload to one subscription. It returns ErrGone when the
// endpoint is no longer valid so the caller can prune it.
func (s *Sender) Send(ctx context.Context, sub Subscription, payload []byte) error {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	resp, err := wp.SendNotificationWithContext(ctx, payload, &wp.Subscription{
		Endpoint: sub.Endpoint,
		Keys:     wp.Keys{P256dh: sub.P256dh, Auth: sub.Auth},
	}, &wp.Options{
		Subscriber:      s.subscriber,
		VAPIDPublicKey:  s.publicKey,
		VAPIDPrivateKey: s.privateKey,
		TTL:             s.ttl,
		Urgency:         s.urgency,
	})
	if err != nil {
		return fmt.Errorf("send push: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
		return ErrGone
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("send push: unexpected status %d", resp.StatusCode)
	}

	return nil
}
