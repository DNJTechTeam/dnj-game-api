package repositories

import (
	"context"
	"errors"
	"testing"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	taskEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/task/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedUser(t *testing.T, ctx context.Context, email string) *entities.User {
	t.Helper()
	user, err := TestSuite.UserRepository.Create(ctx, &entities.User{
		Email:    email,
		Name:     "Test User",
		Password: "Password@#1234",
	})
	require.NoError(t, err)
	return user
}

func TestTaskRepository_CRUD(t *testing.T) {
	ctx := context.Background()
	TestSuite.TruncateTable(t, &models.Task{})
	TestSuite.TruncateTable(t, &models.User{})

	repo := NewTaskRepository(TestSuite.DbConn)
	user := seedUser(t, ctx, "task-repo@example.com")

	// Create
	task, err := repo.Create(ctx, &taskEntities.Task{
		UserID: user.ID,
		Title:  "Repo task",
		Status: taskEntities.TaskStatusPending,
	})
	require.NoError(t, err)
	require.NotNil(t, task)
	assert.NotZero(t, task.ID)

	// FindByIDAndUser — found
	found, err := repo.FindByIDAndUser(ctx, task.ID, user.ID)
	require.NoError(t, err)
	assert.Equal(t, task.ID, found.ID)

	// FindAllByUser
	all, err := repo.FindAllByUser(ctx, user.ID)
	require.NoError(t, err)
	assert.Len(t, all, 1)

	// Update
	task.Title = "Updated"
	updated, err := repo.Update(ctx, task)
	require.NoError(t, err)
	assert.Equal(t, "Updated", updated.Title)

	// Delete — success
	err = repo.Delete(ctx, task.ID, user.ID)
	require.NoError(t, err)

	// Delete — not found
	err = repo.Delete(ctx, task.ID, user.ID)
	assert.ErrorIs(t, err, appErrors.ErrNotFound)
}

func TestTaskRepository_FindByIDAndUser_NotFound(t *testing.T) {
	ctx := context.Background()
	TestSuite.TruncateTable(t, &models.Task{})

	repo := NewTaskRepository(TestSuite.DbConn)

	_, err := repo.FindByIDAndUser(ctx, 9999999, 1)
	assert.ErrorIs(t, err, appErrors.ErrNotFound)
}

// ──── Error branches — go-sqlmock ────────────────────────────────────────────

func TestTaskRepository_Create_Error(t *testing.T) {
	gormDB, mock := newMockDB(t)
	repo := NewTaskRepository(gormDB)
	mock.ExpectBegin().WillReturnError(errors.New("conn failed"))

	_, err := repo.Create(context.Background(), &taskEntities.Task{Title: "t"})
	assert.ErrorIs(t, err, appErrors.InternalError)
}

func TestTaskRepository_FindAllByUser_Error(t *testing.T) {
	gormDB, mock := newMockDB(t)
	repo := NewTaskRepository(gormDB)
	mock.ExpectQuery(`SELECT`).WillReturnError(errors.New("db failure"))

	_, err := repo.FindAllByUser(context.Background(), 1)
	assert.ErrorIs(t, err, appErrors.InternalError)
}

func TestTaskRepository_Update_Error(t *testing.T) {
	gormDB, mock := newMockDB(t)
	repo := NewTaskRepository(gormDB)
	mock.ExpectBegin().WillReturnError(errors.New("conn failed"))

	_, err := repo.Update(context.Background(), &taskEntities.Task{})
	assert.ErrorIs(t, err, appErrors.InternalError)
}

func TestTaskRepository_Delete_Error(t *testing.T) {
	gormDB, mock := newMockDB(t)
	repo := NewTaskRepository(gormDB)
	mock.ExpectBegin().WillReturnError(errors.New("conn failed"))

	err := repo.Delete(context.Background(), 1, 1)
	assert.ErrorIs(t, err, appErrors.InternalError)
}
