package interfaces

import "context"

type SubscriptionWebhookServiceInterface interface {
	// Ingest stores the raw payload and upserts a pending verification code
	// for the subscriber it identifies.
	Ingest(ctx context.Context, rawPayload map[string]any) error
}
