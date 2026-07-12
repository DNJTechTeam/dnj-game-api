// Package webhook holds translators that normalize external
// subscription-platform webhook payloads into the shape the application
// understands. The real webhook payload format is not known yet, so
// NaivePayloadTranslator is a placeholder: it expects a flat JSON object at
// the root with the keys below. Swap this implementation (and its Wire
// provider) once the real payload format is confirmed — nothing else in the
// application needs to change, since callers only depend on
// WebhookPayloadTranslatorInterface.
package webhook

import (
	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/app/interfaces"
)

type NaivePayloadTranslator struct{}

func NewNaivePayloadTranslator() interfaces.WebhookPayloadTranslatorInterface {
	return &NaivePayloadTranslator{}
}

func (t *NaivePayloadTranslator) Translate(payload map[string]any) (*interfaces.TranslatedSubscription, error) {
	email := stringField(payload, "email")
	if email == "" {
		return nil, appErrors.NewError("Payload de webhook inválido.", []*appErrors.FieldError{
			appErrors.NewFieldError("email", "email não informado no payload"),
		})
	}

	return &interfaces.TranslatedSubscription{
		Email:       email,
		Name:        stringField(payload, "name"),
		MobilePhone: stringField(payload, "mobilePhone"),
		Document:    stringField(payload, "document"),
		Group:       stringField(payload, "group"),
	}, nil
}

func stringField(payload map[string]any, key string) string {
	value, ok := payload[key]
	if !ok {
		return ""
	}
	str, ok := value.(string)
	if !ok {
		return ""
	}
	return str
}
