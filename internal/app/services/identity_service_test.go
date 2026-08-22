package services

import (
	"context"
	"sync"
	"testing"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	appInterfaces "github.com/dnjtechteam/dnj-game-api/internal/app/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	groupEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/group/entities"
	userEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeGoogleVerifier struct {
	payload *appInterfaces.GooglePayload
	err     error
}

func (f *fakeGoogleVerifier) Verify(context.Context, string, string) (*appInterfaces.GooglePayload, error) {
	return f.payload, f.err
}

func setupIdentityServiceTest(t *testing.T, payload *appInterfaces.GooglePayload) appInterfaces.IdentityServiceInterface {
	t.Helper()
	TestSuite.DefaultSetup(t)
	for _, model := range []interface{ TableName() string }{
		&models.RefreshSession{}, &models.GoogleIdentity{}, &models.User{}, &models.Group{},
	} {
		TestSuite.TruncateTable(t, model)
	}
	return NewIdentityService(
		TestSuite.BaseService, TestSuite.UserRepository, TestSuite.GroupRepository,
		TestSuite.GoogleIdentityRepository, TestSuite.RefreshSessionRepository,
		NewJwtService(TestSuite.BaseService), &fakeGoogleVerifier{payload: payload},
	)
}

func verifiedGooglePayload(subject, email string) *appInterfaces.GooglePayload {
	return &appInterfaces.GooglePayload{
		Issuer: "https://accounts.google.com", Audience: "test-google-client", Subject: subject,
		Email: email, EmailVerified: true, Name: "Ana Google",
	}
}

func TestIdentityService_AuthenticateGoogle(t *testing.T) {
	t.Run("creates an incomplete profile and a hashed refresh session", func(t *testing.T) {
		// given
		service := setupIdentityServiceTest(t, verifiedGooglePayload("google-sub-1", "ANA@example.com"))

		// when
		response, err := service.AuthenticateGoogle(TestSuite.Ctx, &messages.GoogleAuthRequestDTO{IDToken: "verified-token"})

		// then
		require.NoError(t, err)
		assert.True(t, response.OnboardingRequired)
		assert.False(t, response.User.OnboardingComplete)
		assert.Equal(t, "ana@example.com", response.User.Email)
		assert.NotEmpty(t, response.AccessToken)
		assert.NotEmpty(t, response.RefreshToken)
		var stored models.RefreshSession
		require.NoError(t, TestSuite.DbConn.First(&stored).Error)
		assert.NotEqual(t, response.RefreshToken, stored.TokenHash)
		assert.Len(t, stored.TokenHash, 64)
	})

	t.Run("links a verified Google subject to the existing verified email account", func(t *testing.T) {
		// given
		service := setupIdentityServiceTest(t, verifiedGooglePayload("google-sub-existing", "existing@example.com"))
		existing, err := TestSuite.UserRepository.Create(TestSuite.Ctx, &userEntities.User{Email: "existing@example.com", Name: "Existing", Role: userEntities.RoleDefault})
		require.NoError(t, err)

		// when
		response, authErr := service.AuthenticateGoogle(TestSuite.Ctx, &messages.GoogleAuthRequestDTO{IDToken: "verified-token"})

		// then
		require.NoError(t, authErr)
		assert.Equal(t, messages.Uint64StringFromUint64(existing.ID), response.User.ID)
	})

	t.Run("rejects a changed email for an already linked Google subject", func(t *testing.T) {
		// given
		service := setupIdentityServiceTest(t, verifiedGooglePayload("google-sub-conflict", "first@example.com"))
		_, err := service.AuthenticateGoogle(TestSuite.Ctx, &messages.GoogleAuthRequestDTO{IDToken: "first"})
		require.NoError(t, err)
		service = NewIdentityService(
			TestSuite.BaseService, TestSuite.UserRepository, TestSuite.GroupRepository,
			TestSuite.GoogleIdentityRepository, TestSuite.RefreshSessionRepository,
			NewJwtService(TestSuite.BaseService), &fakeGoogleVerifier{payload: verifiedGooglePayload("google-sub-conflict", "attacker@example.com")},
		)

		// when
		_, conflictErr := service.AuthenticateGoogle(TestSuite.Ctx, &messages.GoogleAuthRequestDTO{IDToken: "second"})

		// then
		var apiErr *appErrors.APIServiceError
		require.ErrorAs(t, conflictErr, &apiErr)
		assert.Equal(t, "IDENTITY_CONFLICT", apiErr.Code)
	})
}

func TestIdentityService_RefreshRotationAndReuse(t *testing.T) {
	// given
	service := setupIdentityServiceTest(t, verifiedGooglePayload("google-refresh", "refresh@example.com"))
	login, err := service.AuthenticateGoogle(TestSuite.Ctx, &messages.GoogleAuthRequestDTO{IDToken: "verified"})
	require.NoError(t, err)

	// when
	rotated, rotateErr := service.Refresh(TestSuite.Ctx, login.RefreshToken)
	_, reuseErr := service.Refresh(TestSuite.Ctx, login.RefreshToken)
	_, revokedFamilyErr := service.Refresh(TestSuite.Ctx, rotated.RefreshToken)

	// then
	require.NoError(t, rotateErr)
	assert.NotEqual(t, login.RefreshToken, rotated.RefreshToken)
	var apiErr *appErrors.APIServiceError
	require.ErrorAs(t, reuseErr, &apiErr)
	assert.Equal(t, "REFRESH_TOKEN_REUSE", apiErr.Code)
	require.Error(t, revokedFamilyErr)
	var reused models.RefreshSession
	require.NoError(t, TestSuite.DbConn.Where("token_hash = ?", tokenHash(login.RefreshToken)).First(&reused).Error)
	assert.NotNil(t, reused.ReuseDetectedAt)
}

func TestIdentityService_ConcurrentRefreshDetectsReuse(t *testing.T) {
	// given
	service := setupIdentityServiceTest(t, verifiedGooglePayload("google-concurrent", "concurrent@example.com"))
	login, err := service.AuthenticateGoogle(TestSuite.Ctx, &messages.GoogleAuthRequestDTO{IDToken: "verified"})
	require.NoError(t, err)
	results := make(chan error, 2)
	var waitGroup sync.WaitGroup
	waitGroup.Add(2)

	// when
	for range 2 {
		go func() {
			defer waitGroup.Done()
			_, refreshErr := service.Refresh(TestSuite.Ctx, login.RefreshToken)
			results <- refreshErr
		}()
	}
	waitGroup.Wait()
	close(results)

	// then
	successes := 0
	failures := 0
	for refreshErr := range results {
		if refreshErr == nil {
			successes++
		} else {
			failures++
		}
	}
	assert.Equal(t, 1, successes)
	assert.Equal(t, 1, failures)
}

func TestIdentityService_CompleteOnboardingAndCurrent(t *testing.T) {
	// given
	service := setupIdentityServiceTest(t, verifiedGooglePayload("google-onboarding", "onboarding@example.com"))
	group, err := TestSuite.GroupRepository.Create(TestSuite.Ctx, &groupEntities.Group{Name: "Grupo Teste"})
	require.NoError(t, err)
	login, err := service.AuthenticateGoogle(TestSuite.Ctx, &messages.GoogleAuthRequestDTO{IDToken: "verified"})
	require.NoError(t, err)
	ctx := TestSuite.ContextWithUser(login.User.ID.Uint64())

	// when
	completed, completeErr := service.CompleteOnboarding(ctx, &messages.CompleteOnboardingRequestDTO{
		Document: "529.982.247-25", MobilePhone: "+55 41 99999-0000", GroupID: messages.Uint64StringFromUint64(group.ID),
	})
	current, currentErr := service.Current(ctx)

	// then
	require.NoError(t, completeErr)
	require.NoError(t, currentErr)
	assert.False(t, completed.OnboardingRequired)
	assert.True(t, current.User.OnboardingComplete)
	assert.Equal(t, "***.***.*47-25", current.User.DocumentMasked)
	var stored models.User
	require.NoError(t, TestSuite.DbConn.First(&stored, login.User.ID.Uint64()).Error)
	assert.Empty(t, stored.Document)
	assert.NotContains(t, stored.DocumentHash, "52998224725")
	assert.Len(t, stored.DocumentHash, 64)
}

func TestIdentityService_RejectsDuplicateDocument(t *testing.T) {
	// given
	firstService := setupIdentityServiceTest(t, verifiedGooglePayload("google-document-1", "first-document@example.com"))
	group, err := TestSuite.GroupRepository.Create(TestSuite.Ctx, &groupEntities.Group{Name: "Grupo Documento"})
	require.NoError(t, err)
	firstLogin, err := firstService.AuthenticateGoogle(TestSuite.Ctx, &messages.GoogleAuthRequestDTO{IDToken: "first"})
	require.NoError(t, err)
	request := &messages.CompleteOnboardingRequestDTO{
		Document: "52998224725", MobilePhone: "5541999990000", GroupID: messages.Uint64StringFromUint64(group.ID),
	}
	_, err = firstService.CompleteOnboarding(TestSuite.ContextWithUser(firstLogin.User.ID.Uint64()), request)
	require.NoError(t, err)
	secondService := NewIdentityService(
		TestSuite.BaseService, TestSuite.UserRepository, TestSuite.GroupRepository,
		TestSuite.GoogleIdentityRepository, TestSuite.RefreshSessionRepository,
		NewJwtService(TestSuite.BaseService), &fakeGoogleVerifier{payload: verifiedGooglePayload("google-document-2", "second-document@example.com")},
	)
	secondLogin, err := secondService.AuthenticateGoogle(TestSuite.Ctx, &messages.GoogleAuthRequestDTO{IDToken: "second"})
	require.NoError(t, err)

	// when
	_, duplicateErr := secondService.CompleteOnboarding(TestSuite.ContextWithUser(secondLogin.User.ID.Uint64()), request)

	// then
	var apiErr *appErrors.APIServiceError
	require.ErrorAs(t, duplicateErr, &apiErr)
	assert.Equal(t, "DOCUMENT_ALREADY_LINKED", apiErr.Code)
}

func TestIdentityService_LogoutRevokesWithoutFalseReuseEvent(t *testing.T) {
	// given
	service := setupIdentityServiceTest(t, verifiedGooglePayload("google-logout", "logout@example.com"))
	login, err := service.AuthenticateGoogle(TestSuite.Ctx, &messages.GoogleAuthRequestDTO{IDToken: "verified"})
	require.NoError(t, err)

	// when
	logoutErr := service.Logout(TestSuite.Ctx, login.RefreshToken)
	unknownErr := service.Logout(TestSuite.Ctx, "unknown-token")

	// then
	require.NoError(t, logoutErr)
	require.NoError(t, unknownErr)
	var session models.RefreshSession
	require.NoError(t, TestSuite.DbConn.Where("token_hash = ?", tokenHash(login.RefreshToken)).First(&session).Error)
	assert.NotNil(t, session.RevokedAt)
	assert.Nil(t, session.ReuseDetectedAt)
}
