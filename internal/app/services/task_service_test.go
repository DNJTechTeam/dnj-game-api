package services

import (
	"context"
	"testing"
	"time"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/app/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	taskEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/task/entities"
	uEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/common"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTaskService() interfaces.TaskServiceInterface {
	return NewTaskService(TestSuite.BaseService, TestSuite.TaskRepository)
}

// seedTaskUser creates a user directly and returns it together with a context
// carrying its identity (as AuthenticationMiddleware would).
func seedTaskUser(t *testing.T, email string) (*uEntities.User, context.Context) {
	t.Helper()
	user, err := TestSuite.UserRepository.Create(TestSuite.Ctx, &uEntities.User{
		Email:       email,
		Name:        "Task User",
		Password:    common.GenerateRandomPassword(),
		MobilePhone: "41992395568",
	})
	require.NoError(t, err)
	return user, TestSuite.ContextWithUser(user.ID)
}

func TestTaskService_Create(t *testing.T) {
	TestSuite.DefaultSetup(t)
	TestSuite.TruncateTable(t, &models.Task{})
	TestSuite.TruncateTable(t, &models.User{})

	service := newTaskService()
	_, ctx := seedTaskUser(t, "creator@example.com")

	t.Run("Valid task", func(t *testing.T) {
		due := time.Now().Add(48 * time.Hour)
		response, err := service.Create(ctx, &messages.CreateTaskRequestDTO{
			Title:       "Write the docs",
			Description: "Document the new resource flow",
			DueDate:     &due,
		})
		require.NoError(t, err)
		require.NotNil(t, response)
		assert.NotZero(t, response.ID.Uint64())
		assert.Equal(t, "Write the docs", response.Title)
		assert.Equal(t, string(taskEntities.TaskStatusPending), response.Status)
	})

	t.Run("Title too short", func(t *testing.T) {
		response, err := service.Create(ctx, &messages.CreateTaskRequestDTO{Title: "no"})
		require.Error(t, err)
		require.Nil(t, response)
		assert.IsType(t, &appErrors.Error{}, err)
	})
}

func TestTaskService_ListIsOwnerScoped(t *testing.T) {
	TestSuite.DefaultSetup(t)
	TestSuite.TruncateTable(t, &models.Task{})
	TestSuite.TruncateTable(t, &models.User{})

	service := newTaskService()
	_, ctxA := seedTaskUser(t, "owner-a@example.com")
	_, ctxB := seedTaskUser(t, "owner-b@example.com")

	_, err := service.Create(ctxA, &messages.CreateTaskRequestDTO{Title: "Task of A"})
	require.NoError(t, err)
	_, err = service.Create(ctxB, &messages.CreateTaskRequestDTO{Title: "Task of B"})
	require.NoError(t, err)

	listA, err := service.List(ctxA)
	require.NoError(t, err)
	require.Len(t, listA, 1)
	assert.Equal(t, "Task of A", listA[0].Title)

	listB, err := service.List(ctxB)
	require.NoError(t, err)
	require.Len(t, listB, 1)
	assert.Equal(t, "Task of B", listB[0].Title)
}

func TestTaskService_GetCannotReadOthers(t *testing.T) {
	TestSuite.DefaultSetup(t)
	TestSuite.TruncateTable(t, &models.Task{})
	TestSuite.TruncateTable(t, &models.User{})

	service := newTaskService()
	_, ctxA := seedTaskUser(t, "a@example.com")
	_, ctxB := seedTaskUser(t, "b@example.com")

	created, err := service.Create(ctxA, &messages.CreateTaskRequestDTO{Title: "Private task"})
	require.NoError(t, err)

	// Owner can read it.
	got, err := service.Get(ctxA, created.ID.Uint64())
	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)

	// A different user gets ErrNotFound.
	_, err = service.Get(ctxB, created.ID.Uint64())
	require.ErrorIs(t, err, appErrors.ErrNotFound)
}

func TestTaskService_UpdateStatus(t *testing.T) {
	TestSuite.DefaultSetup(t)
	TestSuite.TruncateTable(t, &models.Task{})
	TestSuite.TruncateTable(t, &models.User{})

	service := newTaskService()
	_, ctx := seedTaskUser(t, "status@example.com")

	created, err := service.Create(ctx, &messages.CreateTaskRequestDTO{Title: "Status task"})
	require.NoError(t, err)

	updated, err := service.UpdateStatus(ctx, created.ID.Uint64(), &messages.UpdateTaskStatusRequestDTO{Status: "done"})
	require.NoError(t, err)
	assert.Equal(t, string(taskEntities.TaskStatusDone), updated.Status)

	_, err = service.UpdateStatus(ctx, created.ID.Uint64(), &messages.UpdateTaskStatusRequestDTO{Status: "INVALID"})
	require.Error(t, err)
}

func TestTaskService_Delete(t *testing.T) {
	TestSuite.DefaultSetup(t)
	TestSuite.TruncateTable(t, &models.Task{})
	TestSuite.TruncateTable(t, &models.User{})

	service := newTaskService()
	_, ctx := seedTaskUser(t, "delete@example.com")

	created, err := service.Create(ctx, &messages.CreateTaskRequestDTO{Title: "Delete me"})
	require.NoError(t, err)

	require.NoError(t, service.Delete(ctx, created.ID.Uint64()))

	_, err = service.Get(ctx, created.ID.Uint64())
	require.ErrorIs(t, err, appErrors.ErrNotFound)

	// Deleting again returns ErrNotFound.
	require.ErrorIs(t, service.Delete(ctx, created.ID.Uint64()), appErrors.ErrNotFound)
}
