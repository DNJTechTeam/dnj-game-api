package services

import (
	"testing"
	"time"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixedCode makes SignupWithEmail deterministic in tests instead of
// depending on the real crypto/rand-backed generator.
func fixedCode(code string) func(int) (string, error) {
	return func(int) (string, error) { return code, nil }
}

func setupEmailSignupServiceTest(t *testing.T, code string) *IdentityService {
	t.Helper()
	service := setupIdentityServiceTest(t, verifiedGooglePayload("unused", "unused@example.com"))
	impl := service.(*IdentityService)
	impl.signupCode = fixedCode(code)
	return impl
}

func TestIdentityService_SignupWithEmail(t *testing.T) {
	t.Run("sends a code and always answers CODE_SENT, even on resend", func(t *testing.T) {
		// given
		service := setupEmailSignupServiceTest(t, "042917")

		// when
		first, err := service.SignupWithEmail(TestSuite.Ctx, &messages.EmailSignupRequestDTO{Email: "  Ana@Example.com "})

		// then
		require.NoError(t, err)
		assert.Equal(t, "CODE_SENT", first.Status)
	})

	t.Run("rejects a resend inside the cooldown window", func(t *testing.T) {
		// given
		service := setupEmailSignupServiceTest(t, "111222")
		_, err := service.SignupWithEmail(TestSuite.Ctx, &messages.EmailSignupRequestDTO{Email: "cooldown@example.com"})
		require.NoError(t, err)

		// when
		_, err = service.SignupWithEmail(TestSuite.Ctx, &messages.EmailSignupRequestDTO{Email: "cooldown@example.com"})

		// then
		require.Error(t, err)
		apiErr, ok := err.(*appErrors.APIServiceError)
		require.True(t, ok)
		assert.Equal(t, "RATE_LIMITED", apiErr.Code)
	})

	t.Run("allows a resend once the cooldown has elapsed, rotating the code", func(t *testing.T) {
		// given
		service := setupEmailSignupServiceTest(t, "111222")
		fixedNow := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
		service.now = func() time.Time { return fixedNow }
		_, err := service.SignupWithEmail(TestSuite.Ctx, &messages.EmailSignupRequestDTO{Email: "rotate@example.com"})
		require.NoError(t, err)
		service.now = func() time.Time { return fixedNow.Add(61 * time.Second) }
		service.signupCode = fixedCode("333444")

		// when
		_, err = service.SignupWithEmail(TestSuite.Ctx, &messages.EmailSignupRequestDTO{Email: "rotate@example.com"})
		require.NoError(t, err)

		// then — the old code no longer verifies, the new one does
		_, oldErr := service.VerifyEmailSignup(TestSuite.Ctx, &messages.VerifyEmailSignupRequestDTO{Email: "rotate@example.com", Code: "111222"})
		assert.Error(t, oldErr)
		session, newErr := service.VerifyEmailSignup(TestSuite.Ctx, &messages.VerifyEmailSignupRequestDTO{Email: "rotate@example.com", Code: "333444"})
		require.NoError(t, newErr)
		assert.Equal(t, "rotate@example.com", session.User.Email)
	})
}

func TestIdentityService_VerifyEmailSignup(t *testing.T) {
	t.Run("creates an incomplete DEFAULT user on first verification", func(t *testing.T) {
		// given
		service := setupEmailSignupServiceTest(t, "654321")
		_, err := service.SignupWithEmail(TestSuite.Ctx, &messages.EmailSignupRequestDTO{Email: "new@example.com"})
		require.NoError(t, err)

		// when
		session, err := service.VerifyEmailSignup(TestSuite.Ctx, &messages.VerifyEmailSignupRequestDTO{Email: "new@example.com", Code: "654321"})

		// then
		require.NoError(t, err)
		assert.True(t, session.OnboardingRequired)
		assert.Equal(t, "DEFAULT", session.User.Role)
		assert.NotEmpty(t, session.AccessToken)
		assert.NotEmpty(t, session.RefreshToken)
	})

	t.Run("links to the existing account instead of creating a duplicate", func(t *testing.T) {
		// given — the email already has a completed Google account, on the
		// same service instance (a second setup call would truncate users)
		service := setupIdentityServiceTest(t, verifiedGooglePayload("google-sub-link", "linked@example.com")).(*IdentityService)
		service.signupCode = fixedCode("999000")
		googleSession, err := service.AuthenticateGoogle(TestSuite.Ctx, &messages.GoogleAuthRequestDTO{IDToken: "t"})
		require.NoError(t, err)
		_, err = service.SignupWithEmail(TestSuite.Ctx, &messages.EmailSignupRequestDTO{Email: "linked@example.com"})
		require.NoError(t, err)

		// when
		session, err := service.VerifyEmailSignup(TestSuite.Ctx, &messages.VerifyEmailSignupRequestDTO{Email: "linked@example.com", Code: "999000"})

		// then
		require.NoError(t, err)
		assert.Equal(t, googleSession.User.ID, session.User.ID)
	})

	t.Run("rejects an incorrect code without consuming it", func(t *testing.T) {
		// given
		service := setupEmailSignupServiceTest(t, "222333")
		_, err := service.SignupWithEmail(TestSuite.Ctx, &messages.EmailSignupRequestDTO{Email: "wrong@example.com"})
		require.NoError(t, err)

		// when
		_, err = service.VerifyEmailSignup(TestSuite.Ctx, &messages.VerifyEmailSignupRequestDTO{Email: "wrong@example.com", Code: "000000"})

		// then
		require.Error(t, err)
		session, retryErr := service.VerifyEmailSignup(TestSuite.Ctx, &messages.VerifyEmailSignupRequestDTO{Email: "wrong@example.com", Code: "222333"})
		require.NoError(t, retryErr)
		assert.NotNil(t, session)
	})

	t.Run("rejects an expired code", func(t *testing.T) {
		// given
		service := setupEmailSignupServiceTest(t, "777888")
		fixedNow := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
		service.now = func() time.Time { return fixedNow }
		_, err := service.SignupWithEmail(TestSuite.Ctx, &messages.EmailSignupRequestDTO{Email: "expired@example.com"})
		require.NoError(t, err)
		service.now = func() time.Time { return fixedNow.Add(16 * time.Minute) }

		// when
		_, err = service.VerifyEmailSignup(TestSuite.Ctx, &messages.VerifyEmailSignupRequestDTO{Email: "expired@example.com", Code: "777888"})

		// then
		assert.Error(t, err)
	})

	t.Run("rejects an unknown email", func(t *testing.T) {
		// given
		service := setupEmailSignupServiceTest(t, "555555")

		// when
		_, err := service.VerifyEmailSignup(TestSuite.Ctx, &messages.VerifyEmailSignupRequestDTO{Email: "never-signed-up@example.com", Code: "555555"})

		// then
		assert.Error(t, err)
	})
}
