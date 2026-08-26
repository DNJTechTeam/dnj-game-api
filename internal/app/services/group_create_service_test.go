package services

import (
	"testing"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	userEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroupService_CreateGroup(t *testing.T) {
	t.Run("any authenticated user creates a group, self-service", func(t *testing.T) {
		// given
		TestSuite.DefaultSetup(t)
		resetIteration3GroupTables(t)
		service := newGroupService()
		_, playerCtx := seedAdminInstallationUser(t, "player-creates@example.com", userEntities.RoleDefault, true)

		// when
		group, err := service.CreateGroup(playerCtx, &messages.CreateGroupRequestDTO{Name: "  Turma Nova  "})

		// then
		require.NoError(t, err)
		assert.Equal(t, "Turma Nova", group.GroupName)
		assert.NotZero(t, group.ID)
	})

	t.Run("rejects an unauthenticated actor", func(t *testing.T) {
		// given
		TestSuite.DefaultSetup(t)
		resetIteration3GroupTables(t)
		service := newGroupService()

		// when
		_, err := service.CreateGroup(TestSuite.Ctx, &messages.CreateGroupRequestDTO{Name: "Turma Sem Login"})

		// then
		require.Error(t, err)
	})

	t.Run("rejects a duplicate name, case-insensitive", func(t *testing.T) {
		// given
		TestSuite.DefaultSetup(t)
		resetIteration3GroupTables(t)
		service := newGroupService()
		_, playerCtx := seedAdminInstallationUser(t, "player-dup@example.com", userEntities.RoleDefault, true)
		_, err := service.CreateGroup(playerCtx, &messages.CreateGroupRequestDTO{Name: "Turma Duplicada"})
		require.NoError(t, err)

		// when
		_, dupErr := service.CreateGroup(playerCtx, &messages.CreateGroupRequestDTO{Name: "turma duplicada"})

		// then
		var apiErr *appErrors.APIServiceError
		require.ErrorAs(t, dupErr, &apiErr)
		assert.Equal(t, "GROUP_NAME_TAKEN", apiErr.Code)
	})

	t.Run("rejects an empty name", func(t *testing.T) {
		// given
		TestSuite.DefaultSetup(t)
		resetIteration3GroupTables(t)
		service := newGroupService()
		_, playerCtx := seedAdminInstallationUser(t, "player-empty@example.com", userEntities.RoleDefault, true)

		// when
		_, err := service.CreateGroup(playerCtx, &messages.CreateGroupRequestDTO{Name: "   "})

		// then
		require.Error(t, err)
	})
}
