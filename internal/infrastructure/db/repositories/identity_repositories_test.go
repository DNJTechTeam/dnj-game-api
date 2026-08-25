package repositories

import (
	"testing"
	"time"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	identityEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/identity/entities"
	refreshEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/refreshsession/entities"
	userEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGoogleIdentityRepository_CreateAndFind(t *testing.T) {
	// given
	TestSuite.TruncateTable(t, &models.GoogleIdentity{})
	repository := NewGoogleIdentityRepository(TestSuite.DbConn)

	// when
	created, createErr := repository.Create(TestSuite.Ctx, &identityEntities.GoogleIdentity{UserID: 7, Provider: "google", Subject: "subject", Email: "user@example.com"})
	found, findErr := repository.FindByProviderAndSubject(TestSuite.Ctx, "google", "subject")
	missing, missingErr := repository.FindByProviderAndSubject(TestSuite.Ctx, "google", "missing")

	// then
	require.NoError(t, createErr)
	require.NoError(t, findErr)
	require.NoError(t, missingErr)
	assert.Equal(t, created.ID, found.ID)
	assert.Nil(t, missing)
}

func TestRefreshSessionRepository_RotationAndFamilyRevocation(t *testing.T) {
	// given
	TestSuite.TruncateTable(t, &models.RefreshSession{})
	repository := NewRefreshSessionRepository(TestSuite.DbConn)
	now := time.Now().UTC()
	created, err := repository.Create(TestSuite.Ctx, &refreshEntities.RefreshSession{
		ID: "session", UserID: 1, FamilyID: "family", TokenHash: "hash", ExpiresAt: now.Add(time.Hour), LastUsedAt: now,
	})
	require.NoError(t, err)

	// when
	found, findErr := repository.FindByTokenHashForUpdate(TestSuite.Ctx, "hash")
	found.ReplacedByHash = "replacement"
	updated, updateErr := repository.Update(TestSuite.Ctx, found)
	revokeErr := repository.RevokeFamily(TestSuite.Ctx, "family", true)
	revoked, reloadErr := repository.FindByTokenHashForUpdate(TestSuite.Ctx, "hash")
	_, missingErr := repository.FindByTokenHashForUpdate(TestSuite.Ctx, "missing")

	// then
	require.NoError(t, findErr)
	require.NoError(t, updateErr)
	require.NoError(t, revokeErr)
	require.NoError(t, reloadErr)
	assert.Equal(t, created.ID, updated.ID)
	assert.NotNil(t, revoked.RevokedAt)
	assert.NotNil(t, revoked.ReuseDetectedAt)
	assert.ErrorIs(t, missingErr, appErrors.ErrNotFound)
}

func TestUserRepository_FindByDocumentHash(t *testing.T) {
	// given
	TestSuite.TruncateTable(t, &models.User{})
	user, err := TestSuite.UserRepository.Create(TestSuite.Ctx, &userEntities.User{Email: "document@example.com", Name: "Document", Role: userEntities.RoleDefault, DocumentHash: "hash-1"})
	require.NoError(t, err)

	// when
	found, findErr := TestSuite.UserRepository.FindByDocumentHash(TestSuite.Ctx, "hash-1")
	missing, missingErr := TestSuite.UserRepository.FindByDocumentHash(TestSuite.Ctx, "missing")

	// then
	require.NoError(t, findErr)
	require.NoError(t, missingErr)
	assert.Equal(t, user.ID, found.ID)
	assert.Nil(t, missing)
}
