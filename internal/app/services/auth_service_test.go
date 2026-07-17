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
	webhook := seedSubscriptionWebhook(t, "{}")
	seedVerificationCode(t, webhook.ID, "onboard@example.com", "123456", "", "12345678900")

	t.Run("email not found", func(t *testing.T) {
		err := service.Onboarding(TestSuite.Ctx, &messages.OnboardingRequestDTO{
			Email:    "missing@example.com",
			Document: "12345678900",
		})
		require.Error(t, err)
		assert.IsType(t, &appErrors.Error{}, err)
	})

	t.Run("document mismatch", func(t *testing.T) {
		err := service.Onboarding(TestSuite.Ctx, &messages.OnboardingRequestDTO{
			Email:    "onboard@example.com",
			Document: "00000000000",
		})
		require.Error(t, err)
		assert.IsType(t, &appErrors.Error{}, err)
	})

	t.Run("success", func(t *testing.T) {
		err := service.Onboarding(TestSuite.Ctx, &messages.OnboardingRequestDTO{
			Email:    "onboard@example.com",
			Document: "12345678900",
		})
		require.NoError(t, err)
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
