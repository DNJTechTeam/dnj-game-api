package services

import (
	"fmt"
	"testing"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/app/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newGroupService() interfaces.GroupServiceInterface {
	return NewGroupService(TestSuite.BaseService, TestSuite.GroupRepository)
}

func TestGroupService_Search(t *testing.T) {
	TestSuite.DefaultSetup(t)
	TestSuite.TruncateTable(t, &models.Group{})

	service := newGroupService()

	t.Run("less than 3 characters returns validation error", func(t *testing.T) {
		response, err := service.Search(TestSuite.Ctx, "ab")
		require.Error(t, err)
		assert.Nil(t, response)
		assert.IsType(t, &appErrors.Error{}, err)
	})

	t.Run("returns matching results ordered by name", func(t *testing.T) {
		seedGroup(t, "Grupo Jovens A")
		seedGroup(t, "Grupo Jovens B")
		seedGroup(t, "Outro Grupo")

		response, err := service.Search(TestSuite.Ctx, "jovens")
		require.NoError(t, err)
		require.Len(t, response, 2)
		assert.Equal(t, "Grupo Jovens A", response[0].GroupName)
		assert.Equal(t, "Grupo Jovens B", response[1].GroupName)
	})

	t.Run("limited to 20 results", func(t *testing.T) {
		TestSuite.TruncateTable(t, &models.Group{})
		for i := range 25 {
			seedGroup(t, fmt.Sprintf("Limitado %02d", i))
		}

		response, err := service.Search(TestSuite.Ctx, "Limitado")
		require.NoError(t, err)
		assert.Len(t, response, 20)
	})
}
