package services

import (
	"testing"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/app/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	userEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newUserService() interfaces.UserServiceInterface {
	return NewUserService(TestSuite.BaseService, TestSuite.UserRepository, TestSuite.GroupRepository)
}

func seedPlainUser(t *testing.T, email string) *userEntities.User {
	t.Helper()
	user, err := TestSuite.UserRepository.Create(TestSuite.Ctx, &userEntities.User{
		Email: email,
		Name:  "Plain User",
		Role:  userEntities.RoleDefault,
	})
	require.NoError(t, err)
	return user
}

func TestUserService_UpdateGroup(t *testing.T) {
	TestSuite.DefaultSetup(t)
	TestSuite.TruncateTable(t, &models.User{})
	TestSuite.TruncateTable(t, &models.Group{})

	service := newUserService()

	t.Run("groupId existing links the user", func(t *testing.T) {
		user := seedPlainUser(t, "groupid@example.com")
		group := seedGroup(t, "Grupo Existente")

		response, err := service.UpdateGroup(TestSuite.Ctx, user.ID, &messages.UpdateUserGroupRequestDTO{GroupID: group.ID})
		require.NoError(t, err)
		require.NotNil(t, response.Group)
		assert.Equal(t, group.Name, response.Group.GroupName)
	})

	t.Run("groupId inexistente returns validation error", func(t *testing.T) {
		user := seedPlainUser(t, "badgroupid@example.com")

		response, err := service.UpdateGroup(TestSuite.Ctx, user.ID, &messages.UpdateUserGroupRequestDTO{GroupID: 9999999})
		require.Error(t, err)
		assert.Nil(t, response)
		assert.IsType(t, &appErrors.Error{}, err)
	})

	t.Run("groupName existente reaproveita o grupo", func(t *testing.T) {
		user := seedPlainUser(t, "existingname@example.com")
		group := seedGroup(t, "Grupo Por Nome")

		response, err := service.UpdateGroup(TestSuite.Ctx, user.ID, &messages.UpdateUserGroupRequestDTO{GroupName: "grupo por nome"})
		require.NoError(t, err)
		require.NotNil(t, response.Group)
		assert.Equal(t, group.Name, response.Group.GroupName)
	})

	t.Run("groupName novo cria o grupo", func(t *testing.T) {
		user := seedPlainUser(t, "newname@example.com")

		response, err := service.UpdateGroup(TestSuite.Ctx, user.ID, &messages.UpdateUserGroupRequestDTO{GroupName: "Grupo Recém Criado"})
		require.NoError(t, err)
		require.NotNil(t, response.Group)
		assert.Equal(t, "Grupo Recém Criado", response.Group.GroupName)
	})

	t.Run("nem groupId nem groupName retorna erro", func(t *testing.T) {
		user := seedPlainUser(t, "neither@example.com")

		response, err := service.UpdateGroup(TestSuite.Ctx, user.ID, &messages.UpdateUserGroupRequestDTO{})
		require.Error(t, err)
		assert.Nil(t, response)
	})

	t.Run("user not found returns ErrNotFound", func(t *testing.T) {
		response, err := service.UpdateGroup(TestSuite.Ctx, 9999999, &messages.UpdateUserGroupRequestDTO{GroupName: "Any"})
		require.ErrorIs(t, err, appErrors.ErrNotFound)
		assert.Nil(t, response)
	})
}
