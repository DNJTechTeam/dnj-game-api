package services

import (
	"testing"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/app/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newAuthService() interfaces.AuthServiceInterface {
	return NewAuthService(
		TestSuite.BaseService,
		TestSuite.VerificationCodeRepository,
		TestSuite.UserRepository,
		TestSuite.GroupRepository,
		NewJwtService(TestSuite.BaseService),
		newTestEmailService(),
	)
}

func truncateAuthTables(t *testing.T) {
	t.Helper()
	TestSuite.TruncateTable(t, &models.SubscriptionWebhookVerificationCode{})
	TestSuite.TruncateTable(t, &models.SubscriptionWebhook{})
	TestSuite.TruncateTable(t, &models.User{})
	TestSuite.TruncateTable(t, &models.Group{})
}

func TestAuthService_Onboarding(t *testing.T) {
	TestSuite.DefaultSetup(t)
	truncateAuthTables(t)

	service := newAuthService()

	t.Run("document not found", func(t *testing.T) {
		response, err := service.Onboarding(TestSuite.Ctx, &messages.OnboardingRequestDTO{
			Document: "00000000099",
		})
		require.Error(t, err)
		assert.Nil(t, response)
		assert.IsType(t, &appErrors.Error{}, err)
	})

	t.Run("document found with email already on file sends the code", func(t *testing.T) {
		webhook := seedSubscriptionWebhook(t, "{}")
		seedVerificationCode(t, webhook.ID, "onboard@example.com", "123456", "", "12345678900")

		response, err := service.Onboarding(TestSuite.Ctx, &messages.OnboardingRequestDTO{
			Document: "12345678900",
		})
		require.NoError(t, err)
		require.NotNil(t, response)
		assert.Equal(t, messages.OnboardingStatusCodeSent, response.Status)
		assert.Equal(t, "o***d@example.com", response.Email)
	})

	t.Run("document found without email and request has no email returns EMAIL_REQUIRED", func(t *testing.T) {
		webhook := seedSubscriptionWebhook(t, "{}")
		seedVerificationCode(t, webhook.ID, "", "654321", "", "22200000000")

		response, err := service.Onboarding(TestSuite.Ctx, &messages.OnboardingRequestDTO{
			Document: "22200000000",
		})
		require.NoError(t, err)
		require.NotNil(t, response)
		assert.Equal(t, messages.OnboardingStatusEmailRequired, response.Status)
		assert.Empty(t, response.Email)

		code, err := TestSuite.VerificationCodeRepository.FindByDocument(TestSuite.Ctx, "22200000000")
		require.NoError(t, err)
		assert.Empty(t, code.Email)
	})

	t.Run("document found without email and request provides one backfills and sends the code", func(t *testing.T) {
		webhook := seedSubscriptionWebhook(t, "{}")
		seedVerificationCode(t, webhook.ID, "", "111111", "", "33300000000")

		response, err := service.Onboarding(TestSuite.Ctx, &messages.OnboardingRequestDTO{
			Document: "33300000000",
			Email:    " Companion@Example.COM ",
		})
		require.NoError(t, err)
		require.NotNil(t, response)
		assert.Equal(t, messages.OnboardingStatusCodeSent, response.Status)
		assert.Equal(t, "c***n@example.com", response.Email)

		code, err := TestSuite.VerificationCodeRepository.FindByDocument(TestSuite.Ctx, "33300000000")
		require.NoError(t, err)
		assert.Equal(t, "companion@example.com", code.Email)
	})

	t.Run("document found without email and request email in invalid format returns validation error", func(t *testing.T) {
		webhook := seedSubscriptionWebhook(t, "{}")
		seedVerificationCode(t, webhook.ID, "", "222222", "", "44400000000")

		response, err := service.Onboarding(TestSuite.Ctx, &messages.OnboardingRequestDTO{
			Document: "44400000000",
			Email:    "not-an-email",
		})
		require.Error(t, err)
		assert.Nil(t, response)
		assert.IsType(t, &appErrors.Error{}, err)

		code, err := TestSuite.VerificationCodeRepository.FindByDocument(TestSuite.Ctx, "44400000000")
		require.NoError(t, err)
		assert.Empty(t, code.Email)
	})

	t.Run("document found with email already on file ignores a different email in the request", func(t *testing.T) {
		webhook := seedSubscriptionWebhook(t, "{}")
		seedVerificationCode(t, webhook.ID, "keep-me@example.com", "333333", "", "55500000000")

		response, err := service.Onboarding(TestSuite.Ctx, &messages.OnboardingRequestDTO{
			Document: "55500000000",
			Email:    "someone-else@example.com",
		})
		require.NoError(t, err)
		require.NotNil(t, response)
		assert.Equal(t, "k***e@example.com", response.Email)

		code, err := TestSuite.VerificationCodeRepository.FindByDocument(TestSuite.Ctx, "55500000000")
		require.NoError(t, err)
		assert.Equal(t, "keep-me@example.com", code.Email)
	})
}

func TestAuthService_VerifyCode(t *testing.T) {
	TestSuite.DefaultSetup(t)
	truncateAuthTables(t)

	service := newAuthService()

	t.Run("invalid code", func(t *testing.T) {
		webhook := seedSubscriptionWebhook(t, "{}")
		seedVerificationCode(t, webhook.ID, "invalid-code@example.com", "123456", "", "11111111111")

		response, err := service.VerifyCode(TestSuite.Ctx, &messages.VerificationCodeRequestDTO{
			Email:            "invalid-code@example.com",
			VerificationCode: "000000",
		})
		require.Error(t, err)
		assert.Nil(t, response)
		assert.IsType(t, &appErrors.Error{}, err)
	})

	t.Run("success creates user and resolves group", func(t *testing.T) {
		group := seedGroup(t, "Grupo Jovens A")
		webhook := seedSubscriptionWebhook(t, "{}")
		seedVerificationCode(t, webhook.ID, "verify@example.com", "654321", group.Name, "22222222222")

		response, err := service.VerifyCode(TestSuite.Ctx, &messages.VerificationCodeRequestDTO{
			Email:            "verify@example.com",
			VerificationCode: "654321",
		})
		require.NoError(t, err)
		require.NotNil(t, response)
		assert.NotEmpty(t, response.IdentityToken)
		assert.Equal(t, "verify@example.com", response.Email)
		require.NotNil(t, response.Group)
		assert.Equal(t, group.Name, response.Group.GroupName)
	})

	t.Run("no matching group leaves user without a group", func(t *testing.T) {
		webhook := seedSubscriptionWebhook(t, "{}")
		seedVerificationCode(t, webhook.ID, "nogroup@example.com", "111222", "Grupo Inexistente", "33333333333")

		response, err := service.VerifyCode(TestSuite.Ctx, &messages.VerificationCodeRequestDTO{
			Email:            "nogroup@example.com",
			VerificationCode: "111222",
		})
		require.NoError(t, err)
		require.NotNil(t, response)
		assert.Nil(t, response.Group)
	})

	t.Run("idempotent when user already verified", func(t *testing.T) {
		webhook := seedSubscriptionWebhook(t, "{}")
		seedVerificationCode(t, webhook.ID, "idempotent@example.com", "333444", "", "44444444444")

		first, err := service.VerifyCode(TestSuite.Ctx, &messages.VerificationCodeRequestDTO{
			Email:            "idempotent@example.com",
			VerificationCode: "333444",
		})
		require.NoError(t, err)

		second, err := service.VerifyCode(TestSuite.Ctx, &messages.VerificationCodeRequestDTO{
			Email:            "idempotent@example.com",
			VerificationCode: "333444",
		})
		require.NoError(t, err)
		assert.Equal(t, first.ID, second.ID)
	})
}
