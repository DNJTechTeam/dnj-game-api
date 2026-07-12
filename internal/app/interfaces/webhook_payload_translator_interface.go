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

// WebhookPayloadTranslatorInterface isolates the (currently unknown) shape of
// the external platform's webhook payload from the rest of the application.
// Swap the implementation once the real payload format is known.
type WebhookPayloadTranslatorInterface interface {
	Translate(payload map[string]any) (*TranslatedSubscription, error)
}
