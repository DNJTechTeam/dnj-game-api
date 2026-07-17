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

const groupCustomFormName = "Qual seu Grupo de Jovens / Movimento? (No caso de nome com sigla coloque também o nome completo)"

func newSubscriptionWebhookService() interfaces.SubscriptionWebhookServiceInterface {
	return NewSubscriptionWebhookService(
		TestSuite.BaseService,
		TestSuite.SubscriptionWebhookRepository,
		TestSuite.VerificationCodeRepository,
		TestSuite.GroupRepository,
		webhook.NewOrderPayloadTranslator(),
	)
}

func customForm(name string, value string) map[string]any {
	return map[string]any{"name": name, "value": value}
}

func orderPayload(buyerCPF string, participants ...map[string]any) map[string]any {
	list := make([]any, len(participants))
	for i, p := range participants {
		list[i] = p
	}
	return map[string]any{
		"buyer_cpf":    buyerCPF,
		"participants": list,
	}
}

func participant(name, email, phone, cpf string, customForms ...map[string]any) map[string]any {
	forms := make([]any, len(customForms))
	for i, f := range customForms {
		forms[i] = f
	}
	return map[string]any{
		"name":         name,
		"email":        email,
		"phone":        phone,
		"cpf":          cpf,
		"custom_forms": forms,
	}
}

func TestSubscriptionWebhookService_Ingest(t *testing.T) {
	TestSuite.DefaultSetup(t)
	TestSuite.TruncateTable(t, &models.SubscriptionWebhookVerificationCode{})
	TestSuite.TruncateTable(t, &models.SubscriptionWebhook{})
	TestSuite.TruncateTable(t, &models.Group{})

	service := newSubscriptionWebhookService()

	t.Run("creates a new verification code for a new participant", func(t *testing.T) {
		payload := orderPayload("12345678900",
			participant("New Subscriber", " New-Subscriber@Example.com ", "41999999999", "12345678900"),
		)

		err := service.Ingest(TestSuite.Ctx, payload)
		require.NoError(t, err)

		code, err := TestSuite.VerificationCodeRepository.FindByDocument(TestSuite.Ctx, "12345678900")
		require.NoError(t, err)
		require.NotNil(t, code)
		assert.Len(t, code.VerificationCode, 6)
		assert.Equal(t, "new-subscriber@example.com", code.Email)
	})

	t.Run("re-ingesting the same document rotates the code and updates fields", func(t *testing.T) {
		first := orderPayload("22200000000",
			participant("Original Name", "rotate@example.com", "", "22200000000"),
		)
		err := service.Ingest(TestSuite.Ctx, first)
		require.NoError(t, err)

		firstCode, err := TestSuite.VerificationCodeRepository.FindByDocument(TestSuite.Ctx, "22200000000")
		require.NoError(t, err)

		second := orderPayload("22200000000",
			participant("Updated Name", "rotate@example.com", "", "22200000000"),
		)
		err = service.Ingest(TestSuite.Ctx, second)
		require.NoError(t, err)

		secondCode, err := TestSuite.VerificationCodeRepository.FindByDocument(TestSuite.Ctx, "22200000000")
		require.NoError(t, err)
		assert.NotEqual(t, firstCode.VerificationCode, secondCode.VerificationCode)
		assert.Equal(t, "Updated Name", secondCode.Name)
	})

	t.Run("duplicated email within the same order keeps it only for the buyer", func(t *testing.T) {
		payload := orderPayload("33300000000",
			participant("Buyer Name", "shared@example.com", "", "33300000000"),
			participant("Companion Name", "shared@example.com", "", "33300000001"),
		)

		err := service.Ingest(TestSuite.Ctx, payload)
		require.NoError(t, err)

		buyerCode, err := TestSuite.VerificationCodeRepository.FindByDocument(TestSuite.Ctx, "33300000000")
		require.NoError(t, err)
		require.NotNil(t, buyerCode)
		assert.Equal(t, "shared@example.com", buyerCode.Email)

		companionCode, err := TestSuite.VerificationCodeRepository.FindByDocument(TestSuite.Ctx, "33300000001")
		require.NoError(t, err)
		require.NotNil(t, companionCode)
		assert.Empty(t, companionCode.Email)
	})

	t.Run("missing cpf propagates a validation error without persisting a verification code", func(t *testing.T) {
		payload := orderPayload("44400000000",
			participant("No CPF Here", "no-cpf@example.com", "", ""),
		)

		err := service.Ingest(TestSuite.Ctx, payload)
		require.Error(t, err)
		assert.IsType(t, &appErrors.Error{}, err)

		code, err := TestSuite.VerificationCodeRepository.FindByDocument(TestSuite.Ctx, "")
		require.NoError(t, err)
		assert.Nil(t, code)
	})

	t.Run("group custom form matching an existing group is linked", func(t *testing.T) {
		group := seedGroup(t, "GRUPO EXISTENTE")
		payload := orderPayload("55500000000",
			participant("Grouped Subscriber", "grouped@example.com", "", "55500000000",
				customForm(groupCustomFormName, "grupo existente"),
			),
		)

		err := service.Ingest(TestSuite.Ctx, payload)
		require.NoError(t, err)

		code, err := TestSuite.VerificationCodeRepository.FindByDocument(TestSuite.Ctx, "55500000000")
		require.NoError(t, err)
		require.NotNil(t, code)
		assert.Equal(t, group.Name, code.Group)
	})

	t.Run("group custom form with no matching group is ignored", func(t *testing.T) {
		payload := orderPayload("66600000000",
			participant("Ungrouped Subscriber", "ungrouped@example.com", "", "66600000000",
				customForm(groupCustomFormName, "grupo inexistente"),
			),
		)

		err := service.Ingest(TestSuite.Ctx, payload)
		require.NoError(t, err)

		code, err := TestSuite.VerificationCodeRepository.FindByDocument(TestSuite.Ctx, "66600000000")
		require.NoError(t, err)
		require.NotNil(t, code)
		assert.Empty(t, code.Group)
	})
}
