package repositories

import (
	"context"
	"testing"
	"time"

	activityEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/activity/entities"
	spaceEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/space/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminInstallation_RepositoryCRUDCoverage(t *testing.T) {
	ctx := context.Background()
	TestSuite.TruncateTable(t, &models.ActivityManagerAssignment{})
	TestSuite.TruncateTable(t, &models.Activity{})
	TestSuite.TruncateTable(t, &models.Space{})
	TestSuite.TruncateTable(t, &models.User{})
	now := time.Now().UTC()

	spaceRepo := NewSpaceRepository(TestSuite.DbConn)
	space := &spaceEntities.Space{ID: uuid.NewString(), Slug: "patio-" + uuid.NewString()[:8], Name: "Pátio", CreatedAt: now, UpdatedAt: now}
	createdSpace, err := spaceRepo.Create(ctx, space)
	require.NoError(t, err)
	foundSpace, err := spaceRepo.FindByIDForUpdate(ctx, createdSpace.ID)
	require.NoError(t, err)
	foundSpace.Name = "Pátio central"
	foundSpace.UpdatedAt = now.Add(time.Minute)
	updatedSpace, err := spaceRepo.Update(ctx, foundSpace)
	require.NoError(t, err)
	assert.Equal(t, "Pátio central", updatedSpace.Name)
	spaces, err := spaceRepo.List(ctx, 0)
	require.NoError(t, err)
	assert.Len(t, spaces.Data, 1)

	manager := seedUser(t, ctx, "admin-repository-manager@example.com")
	activityRepo := NewActivityRepository(TestSuite.DbConn)
	activity := &activityEntities.Activity{ID: uuid.NewString(), Slug: "activity-" + uuid.NewString()[:8], Name: "Atividade administrativa", SpaceID: &createdSpace.ID, Kind: activityEntities.KindLive, Status: activityEntities.StatusDraft, CreatedAt: now, UpdatedAt: now}
	createdActivity, err := activityRepo.Create(ctx, activity)
	require.NoError(t, err)
	foundActivity, err := activityRepo.FindByID(ctx, createdActivity.ID)
	require.NoError(t, err)
	_, err = activityRepo.FindByIDForUpdate(ctx, createdActivity.ID)
	require.NoError(t, err)
	foundActivity.Name = "Atividade atualizada"
	foundActivity.UpdatedAt = now.Add(time.Minute)
	_, err = activityRepo.Update(ctx, foundActivity)
	require.NoError(t, err)
	activities, err := activityRepo.List(ctx, 0)
	require.NoError(t, err)
	assert.Len(t, activities.Data, 1)

	created, err := activityRepo.CreateManagerAssignment(ctx, &activityEntities.ManagerAssignment{ActivityID: createdActivity.ID, UserID: manager.ID, CreatedAt: now})
	require.NoError(t, err)
	assert.True(t, created)
	created, err = activityRepo.CreateManagerAssignment(ctx, &activityEntities.ManagerAssignment{ActivityID: createdActivity.ID, UserID: manager.ID, CreatedAt: now})
	require.NoError(t, err)
	assert.False(t, created)
	_, err = activityRepo.FindAuthorizedForUpdate(ctx, createdActivity.ID, manager.ID, false)
	require.NoError(t, err)
	_, err = activityRepo.FindAuthorizedForUpdate(ctx, createdActivity.ID, 0, true)
	require.NoError(t, err)
	managers, err := activityRepo.ListManagers(ctx, createdActivity.ID, 0)
	require.NoError(t, err)
	assert.Len(t, managers.Data, 1)
	count, err := activityRepo.CountManagerAssignments(ctx, manager.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	require.NoError(t, activityRepo.TransitionStatus(ctx, createdActivity.ID, activityEntities.StatusDraft, activityEntities.StatusActive, now.Add(2*time.Minute)))
	assert.Error(t, activityRepo.TransitionStatus(ctx, createdActivity.ID, activityEntities.StatusDraft, activityEntities.StatusActive, now.Add(3*time.Minute)))
	deleted, err := activityRepo.DeleteManagerAssignment(ctx, createdActivity.ID, manager.ID)
	require.NoError(t, err)
	assert.True(t, deleted)
	deleted, err = activityRepo.DeleteManagerAssignment(ctx, createdActivity.ID, manager.ID)
	require.NoError(t, err)
	assert.False(t, deleted)
	managers, err = activityRepo.ListManagers(ctx, createdActivity.ID, 0)
	require.NoError(t, err)
	assert.Empty(t, managers.Data)
}
