package services

import (
	"testing"

	"github.com/dnjtechteam/dnj-game-api/internal/app/interfaces"
	groupEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/group/entities"
	swEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/subscriptionwebhook/entities"
	svcEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/subscriptionwebhookverificationcode/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/email"

	"github.com/stretchr/testify/require"
)

// newTestEmailService returns the real EmailService. SendEmail is a no-op in
// the test environment (see EmailService.SendEmail), so it is safe to use here.
func newTestEmailService() interfaces.EmailServiceInterface {
	return email.NewEmailService()
}

func seedGroup(t *testing.T, name string) *groupEntities.Group {
	t.Helper()
	group, err := TestSuite.GroupRepository.Create(TestSuite.Ctx, &groupEntities.Group{Name: name})
	require.NoError(t, err)
	return group
}

func seedSubscriptionWebhook(t *testing.T, payload string) *swEntities.SubscriptionWebhook {
	t.Helper()
	webhook, err := TestSuite.SubscriptionWebhookRepository.Create(TestSuite.Ctx, &swEntities.SubscriptionWebhook{Payload: payload})
	require.NoError(t, err)
	return webhook
}

func seedVerificationCode(t *testing.T, webhookID uint64, email string, code string, group string) *svcEntities.SubscriptionWebhookVerificationCode {
	t.Helper()
	verificationCode, err := TestSuite.VerificationCodeRepository.Create(TestSuite.Ctx, &svcEntities.SubscriptionWebhookVerificationCode{
		SubscriptionWebhookID: webhookID,
		Email:                 email,
		Name:                  "Test Subscriber",
		MobilePhone:           "41992395568",
		Document:              "12345678900",
		VerificationCode:      code,
		Group:                 group,
	})
	require.NoError(t, err)
	return verificationCode
}
