package entities

// PushSubscription is a browser Web Push subscription for the single app user.
// Endpoint is the push service URL; the keys encrypt the payload.
type PushSubscription struct {
	Endpoint string
	P256dh   string
	Auth     string
}
