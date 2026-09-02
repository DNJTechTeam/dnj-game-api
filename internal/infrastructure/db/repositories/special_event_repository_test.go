package repositories

import (
	"context"
	"testing"
	"time"

	specialEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/specialevent/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedSpecialEventActivity(t *testing.T, userID uint64, name string, now time.Time) string {
	t.Helper()
	id := uuid.NewString()
	require.NoError(t, TestSuite.DbConn.Create(&models.Activity{ID: id, Slug: "special-" + id[:8], Name: name, Kind: "live", Status: "active", CreatedAt: now, UpdatedAt: now}).Error)
	require.NoError(t, TestSuite.DbConn.Create(&models.ActivityManagerAssignment{ActivityID: id, UserID: userID, CreatedAt: now}).Error)
	return id
}

func TestAdminInstallation_SpecialEventRepositoryLifecycle(t *testing.T) {
	ctx := context.Background()
	TestSuite.TruncateTable(t, &models.SpecialEvent{})
	TestSuite.TruncateTable(t, &models.ActivityManagerAssignment{})
	TestSuite.TruncateTable(t, &models.Activity{})
	TestSuite.TruncateTable(t, &models.User{})

	now := time.Now().UTC()
	manager := seedUser(t, ctx, "special-event-manager@example.com")
	activityID := seedSpecialEventActivity(t, manager.ID, "Caça ao tesouro", now)
	repo := NewSpecialEventRepository(TestSuite.DbConn)
	event := &specialEntities.Event{ID: uuid.NewString(), ActivityID: activityID, Title: "Caça ao tesouro", Points: 20, DurationMinutes: 30, Targets: []string{"app", "tv"}, Status: specialEntities.StatusDraft, EndsAt: now.Add(time.Hour), CreatedBy: manager.ID, CreatedAt: now, UpdatedAt: now}
	require.NoError(t, repo.Create(ctx, event))

	listed, err := repo.ListForManager(ctx, manager.ID, false)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, event.ID, listed[0].ID)

	found, err := repo.FindForManager(ctx, event.ID, manager.ID, false, true)
	require.NoError(t, err)
	assert.Equal(t, event.Title, found.Title)

	teaserAt := now.Add(time.Minute)
	event.Status, event.TeaserAt, event.UpdatedAt = specialEntities.StatusTeaser, &teaserAt, teaserAt
	require.NoError(t, repo.Save(ctx, event))
	visible, err := repo.FindVisible(ctx, "app", now)
	require.NoError(t, err)
	assert.Equal(t, specialEntities.StatusTeaser, visible.Status)

	event.Status, event.UpdatedAt = specialEntities.StatusClosed, now.Add(2*time.Minute)
	require.NoError(t, repo.Save(ctx, event))
	listed, err = repo.ListForManager(ctx, manager.ID, true)
	require.NoError(t, err)
	assert.Empty(t, listed)
	_, err = repo.FindVisible(ctx, "app", now)
	assert.Error(t, err)
	assert.Nil(t, specialEventEntity(nil))
}
