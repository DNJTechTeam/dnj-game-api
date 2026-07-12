package services

import (
	"testing"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/app/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/webhook"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newSubscriptionWebhookService() interfaces.SubscriptionWebhookServiceInterface {
	return NewSubscriptionWebhookService(
		TestSuite.BaseService,
		TestSuite.SubscriptionWebhookRepository,
		TestSuite.VerificationCodeRepository,
		webhook.NewNaivePayloadTranslator(),
	)
}

func TestSubscriptionWebhookService_Ingest(t *testing.T) {
	TestSuite.DefaultSetup(t)
	TestSuite.TruncateTable(t, &models.SubscriptionWebhookVerificationCode{})
	TestSuite.TruncateTable(t, &models.SubscriptionWebhook{})

	service := newSubscriptionWebhookService()

	t.Run("creates a new verification code for a new email", func(t *testing.T) {
		err := service.Ingest(TestSuite.Ctx, map[string]any{
			"email":       "new-subscriber@example.com",
			"name":        "New Subscriber",
			"mobilePhone": "41999999999",
			"document":    "12345678900",
			"group":       "Grupo A",
		})
		require.NoError(t, err)

		code, err := TestSuite.VerificationCodeRepository.FindByEmail(TestSuite.Ctx, "new-subscriber@example.com")
		require.NoError(t, err)
		require.NotNil(t, code)
		assert.Len(t, code.VerificationCode, 6)
		assert.Equal(t, "Grupo A", code.Group)
	})

	t.Run("re-ingesting the same email rotates the code and updates fields", func(t *testing.T) {
		err := service.Ingest(TestSuite.Ctx, map[string]any{
			"email": "rotate@example.com",
			"name":  "Original Name",
			"group": "Grupo A",
		})
		require.NoError(t, err)

		first, err := TestSuite.VerificationCodeRepository.FindByEmail(TestSuite.Ctx, "rotate@example.com")
		require.NoError(t, err)

		err = service.Ingest(TestSuite.Ctx, map[string]any{
			"email": "rotate@example.com",
			"name":  "Updated Name",
			"group": "Grupo B",
		})
		require.NoError(t, err)

		second, err := TestSuite.VerificationCodeRepository.FindByEmail(TestSuite.Ctx, "rotate@example.com")
		require.NoError(t, err)
		assert.NotEqual(t, first.VerificationCode, second.VerificationCode)
		assert.Equal(t, "Updated Name", second.Name)
		assert.Equal(t, "Grupo B", second.Group)
	})

	t.Run("translator error propagates without creating a verification code", func(t *testing.T) {
		err := service.Ingest(TestSuite.Ctx, map[string]any{"name": "No email here"})
		require.Error(t, err)
		assert.IsType(t, &appErrors.Error{}, err)

		code, err := TestSuite.VerificationCodeRepository.FindByEmail(TestSuite.Ctx, "")
		require.NoError(t, err)
		assert.Nil(t, code)
	})
}
