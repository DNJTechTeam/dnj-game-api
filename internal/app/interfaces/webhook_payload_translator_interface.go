package interfaces

// TranslatedSubscription is the normalized shape extracted from a
// subscription webhook payload, regardless of its original format.
type TranslatedSubscription struct {
	Email       string
	Name        string
	MobilePhone string
	Document    string
	Group       string
}

// WebhookPayloadTranslatorInterface isolates the shape of the external
// platform's webhook payload from the rest of the application. A single
// webhook payload (an order) can carry multiple participants, so Translate
// returns one TranslatedSubscription per participant.
type WebhookPayloadTranslatorInterface interface {
	Translate(payload map[string]any) ([]*TranslatedSubscription, error)
}
