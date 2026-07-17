// Package webhook holds translators that normalize external
// subscription-platform webhook payloads into the shape the application
// understands.
package webhook

import (
	"fmt"
	"strings"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/app/interfaces"
)

// groupCustomFormName is the exact custom_forms question whose answer feeds
// the Group field of a translated participant.
const groupCustomFormName = "Qual seu Grupo de Jovens / Movimento? (No caso de nome com sigla coloque também o nome completo)"

// OrderPayloadTranslator parses the ticketing platform's order webhook
// payload: an order with buyer data and a list of participants. Each
// participant becomes one TranslatedSubscription.
type OrderPayloadTranslator struct{}

func NewOrderPayloadTranslator() interfaces.WebhookPayloadTranslatorInterface {
	return &OrderPayloadTranslator{}
}

func (t *OrderPayloadTranslator) Translate(payload map[string]any) ([]*interfaces.TranslatedSubscription, error) {
	participantsRaw, ok := payload["participants"].([]any)
	if !ok || len(participantsRaw) == 0 {
		return nil, appErrors.NewError("Payload de webhook inválido.", []*appErrors.FieldError{
			appErrors.NewFieldError("participants", "nenhum participante informado no payload"),
		})
	}

	buyerCPF := stringField(payload, "buyer_cpf")

	translated := make([]*interfaces.TranslatedSubscription, 0, len(participantsRaw))
	for i, raw := range participantsRaw {
		participant, ok := raw.(map[string]any)
		if !ok {
			continue
		}

		document := stringField(participant, "cpf")
		if document == "" {
			return nil, appErrors.NewError("Payload de webhook inválido.", []*appErrors.FieldError{
				appErrors.NewFieldError(fmt.Sprintf("participants[%d].cpf", i), "cpf não informado no participante"),
			})
		}

		translated = append(translated, &interfaces.TranslatedSubscription{
			Email:       normalizeEmail(stringField(participant, "email")),
			Name:        stringField(participant, "name"),
			MobilePhone: stringField(participant, "phone"),
			Document:    document,
			Group:       strings.ToUpper(strings.TrimSpace(customFormValue(participant, groupCustomFormName))),
		})
	}

	deduplicateEmails(translated, buyerCPF)

	return translated, nil
}

// deduplicateEmails blanks out the email of every participant that shares an
// email with another participant in the same payload, except for the buyer
// (matched by document/CPF) — the platform commonly reuses the buyer's email
// across companions' registrations.
func deduplicateEmails(translated []*interfaces.TranslatedSubscription, buyerCPF string) {
	counts := make(map[string]int, len(translated))
	for _, t := range translated {
		if t.Email == "" {
			continue
		}
		counts[t.Email]++
	}

	for _, t := range translated {
		if t.Email != "" && counts[t.Email] > 1 && t.Document != buyerCPF {
			t.Email = ""
		}
	}
}

func customFormValue(participant map[string]any, name string) string {
	formsRaw, ok := participant["custom_forms"].([]any)
	if !ok {
		return ""
	}

	for _, formRaw := range formsRaw {
		form, ok := formRaw.(map[string]any)
		if !ok {
			continue
		}
		if stringField(form, "name") == name {
			return stringField(form, "value")
		}
	}

	return ""
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
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
