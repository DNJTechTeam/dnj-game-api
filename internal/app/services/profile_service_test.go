package services

import (
	"encoding/json"
	"testing"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	userEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupProfileServiceTest(t *testing.T) *ProfileService {
	t.Helper()
	TestSuite.DefaultSetup(t)
	TestSuite.TruncateTable(t, &models.GroupInvite{})
	TestSuite.TruncateTable(t, &models.GroupMembership{})
	TestSuite.TruncateTable(t, &models.User{})
	TestSuite.TruncateTable(t, &models.Group{})
	return NewProfileService(TestSuite.BaseService, TestSuite.UserRepository, TestSuite.GroupRepository).(*ProfileService)
}

func TestProfileService_CurrentAndUpdate(t *testing.T) {
	t.Run("reads a safe current profile and never serializes raw document or hashes", func(t *testing.T) {
		// given
		service := setupProfileServiceTest(t)
		group := seedGroup(t, "Jovens da Luz")
		groupID := group.ID
		user, err := TestSuite.UserRepository.Create(TestSuite.Ctx, &userEntities.User{Email: "ana@example.com", Name: "Ana Souza", MobilePhone: "5541999990000", Document: "52998224725", DocumentHash: "secret-hash", DocumentLast4: "4725", Role: userEntities.RoleDefault, GroupID: &groupID, Points: 20, OnboardingComplete: true})
		require.NoError(t, err)
		ctx := TestSuite.ContextWithUser(user.ID)

		// when
		response, currentErr := service.Current(ctx)
		encoded, marshalErr := json.Marshal(response)

		// then
		require.NoError(t, currentErr)
		require.NoError(t, marshalErr)
		assert.Equal(t, "***.***.*47-25", response.DocumentMasked)
		assert.Equal(t, int64(1), response.RankPosition)
		assert.NotContains(t, string(encoded), "52998224725")
		assert.NotContains(t, string(encoded), "secret-hash")
		assert.NotContains(t, string(encoded), "documentHash")
	})

	t.Run("updates only name and normalized mobile while preserving identity fields", func(t *testing.T) {
		// given
		service := setupProfileServiceTest(t)
		group := seedGroup(t, "Grupo Perfil")
		groupID := group.ID
		user, err := TestSuite.UserRepository.Create(TestSuite.Ctx, &userEntities.User{Email: "fixed@example.com", Name: "Before", MobilePhone: "5541999990000", DocumentHash: "fixed-hash", DocumentLast4: "4725", Role: userEntities.RoleEventManager, GroupID: &groupID, OnboardingComplete: true})
		require.NoError(t, err)
		name, phone := "  Ana Atualizada  ", "+55 (41) 98888-7777"

		// when
		response, updateErr := service.Update(TestSuite.ContextWithUser(user.ID), &messages.UpdateCurrentProfileRequestDTO{Name: &name, MobilePhone: &phone})
		stored, findErr := TestSuite.UserRepository.FindByID(TestSuite.Ctx, user.ID)

		// then
		require.NoError(t, updateErr)
		require.NoError(t, findErr)
		assert.Equal(t, "Ana Atualizada", response.Name)
		assert.Equal(t, "5541988887777", response.MobilePhone)
		assert.Equal(t, "fixed@example.com", stored.Email)
		assert.Equal(t, "fixed-hash", stored.DocumentHash)
		assert.Equal(t, userEntities.RoleEventManager, stored.Role)
		assert.Equal(t, groupID, *stored.GroupID)
	})

	t.Run("persists a profile photo for the Moments feed", func(t *testing.T) {
		// given
		service := setupProfileServiceTest(t)
		user, err := TestSuite.UserRepository.Create(TestSuite.Ctx, &userEntities.User{Email: "avatar@example.com", Name: "Avatar User", Role: userEntities.RoleDefault})
		require.NoError(t, err)
		avatarURL := "data:image/png;base64,aGVsbG8="

		// when
		response, updateErr := service.Update(TestSuite.ContextWithUser(user.ID), &messages.UpdateCurrentProfileRequestDTO{AvatarURL: &avatarURL})

		// then
		require.NoError(t, updateErr)
		require.NotNil(t, response.AvatarURL)
		assert.Equal(t, avatarURL, *response.AvatarURL)
		stored, findErr := TestSuite.UserRepository.FindByID(TestSuite.Ctx, user.ID)
		require.NoError(t, findErr)
		require.NotNil(t, stored.AvatarURL)
		assert.Equal(t, avatarURL, *stored.AvatarURL)
	})
}

func TestProfileService_ValidationAndAuthorization(t *testing.T) {
	// given
	service := setupProfileServiceTest(t)
	user, err := TestSuite.UserRepository.Create(TestSuite.Ctx, &userEntities.User{Email: "validation@example.com", Name: "Valid User", Role: userEntities.RoleDefault})
	require.NoError(t, err)
	ctx := TestSuite.ContextWithUser(user.ID)
	shortName, invalidPhone := "A", "123"

	// when
	_, unauthenticatedErr := service.Current(TestSuite.Ctx)
	_, emptyErr := service.Update(ctx, &messages.UpdateCurrentProfileRequestDTO{})
	_, nameErr := service.Update(ctx, &messages.UpdateCurrentProfileRequestDTO{Name: &shortName})
	_, phoneErr := service.Update(ctx, &messages.UpdateCurrentProfileRequestDTO{MobilePhone: &invalidPhone})

	// then
	for _, err := range []error{unauthenticatedErr, emptyErr, nameErr, phoneErr} {
		var apiErr *appErrors.APIServiceError
		require.ErrorAs(t, err, &apiErr)
	}
}
