package entities

import "time"

// SubscriptionWebhook stores the raw payload received from the external
// event-subscription platform, before any translation/normalization.
type SubscriptionWebhook struct {
	ID        uint64
	Payload   string
	CreatedAt time.Time
}
