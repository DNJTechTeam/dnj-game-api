package repositories

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	activityEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/activity/entities"
	auditEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/operationaudit/entities"
	userEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupIteration4RepositoryTest(t *testing.T) {
	t.Helper()
	for _, model := range []interface{ TableName() string }{&models.OperationAudit{}, &models.ActivityManagerAssignment{}, &models.Activity{}, &models.Space{}, &models.User{}} {
		TestSuite.TruncateTable(t, model)
	}
}

func TestSpaceRepository_List(t *testing.T) {
	// given
	setupIteration4RepositoryTest(t)
	repository := NewSpaceRepository(TestSuite.DbConn)
	now := time.Now().UTC()
	for index := 20; index >= 0; index-- {
		id := uuid.NewString()
		require.NoError(t, TestSuite.DbConn.Create(&models.Space{ID: id, Slug: "space-" + id, Name: string(rune('A' + index)), CreatedAt: now, UpdatedAt: now}).Error)
	}

	// when
	first, firstErr := repository.List(context.Background(), 0)
	second, secondErr := repository.List(context.Background(), 1)

	// then
	require.NoError(t, firstErr)
	require.NoError(t, secondErr)
	require.Len(t, first.Data, 20)
	assert.Equal(t, "A", first.Data[0].Name)
	assert.True(t, first.Pagination.HasNextPage)
	require.Len(t, second.Data, 1)
	assert.Equal(t, "U", second.Data[0].Name)
}

func TestActivityRepository_AssignmentIsolationAndConditionalTransition(t *testing.T) {
	// given
	setupIteration4RepositoryTest(t)
	users := NewUserRepository(TestSuite.DbConn)
	manager, err := users.Create(context.Background(), &userEntities.User{Email: "repo-manager@example.com", Name: "Manager", Role: userEntities.RoleEventManager})
	require.NoError(t, err)
	other, err := users.Create(context.Background(), &userEntities.User{Email: "repo-other@example.com", Name: "Other", Role: userEntities.RoleEventManager})
	require.NoError(t, err)
	activityID := uuid.NewString()
	now := time.Now().UTC()
	require.NoError(t, TestSuite.DbConn.Create(&models.Activity{ID: activityID, Slug: "activity-" + activityID, Name: "Activity", Kind: "competitive", Status: "draft", CreatedAt: now, UpdatedAt: now}).Error)
	require.NoError(t, TestSuite.DbConn.Create(&models.ActivityManagerAssignment{ActivityID: activityID, UserID: manager.ID, CreatedAt: now}).Error)
	repository := NewActivityRepository(TestSuite.DbConn)

	// when
	assigned, assignedErr := repository.FindAuthorizedForUpdate(context.Background(), activityID, manager.ID, false)
	_, outsideErr := repository.FindAuthorizedForUpdate(context.Background(), activityID, other.ID, false)
	global, globalErr := repository.FindAuthorizedForUpdate(context.Background(), activityID, other.ID, true)
	transitionErr := repository.TransitionStatus(context.Background(), activityID, activityEntities.StatusDraft, activityEntities.StatusActive, now.Add(time.Minute))
	staleErr := repository.TransitionStatus(context.Background(), activityID, activityEntities.StatusDraft, activityEntities.StatusPaused, now.Add(2*time.Minute))

	// then
	require.NoError(t, assignedErr)
	require.NoError(t, globalErr)
	assert.Equal(t, activityID, assigned.ID)
	assert.Equal(t, activityID, global.ID)
	assert.ErrorIs(t, outsideErr, appErrors.ErrNotFound)
	require.NoError(t, transitionErr)
	assert.ErrorIs(t, staleErr, appErrors.ErrConflict)
}

func TestOperationAuditRepository_CreateFindAndRejectDuplicateKey(t *testing.T) {
	// given
	setupIteration4RepositoryTest(t)
	users := NewUserRepository(TestSuite.DbConn)
	actor, err := users.Create(context.Background(), &userEntities.User{Email: "repo-audit@example.com", Name: "Audit", Role: userEntities.RoleAdmin})
	require.NoError(t, err)
	repository := NewOperationAuditRepository(TestSuite.DbConn)
	activityID := uuid.NewString()
	key := uuid.NewString()
	metadata := json.RawMessage(`{"fromStatus":"draft","toStatus":"active"}`)
	audit := &auditEntities.OperationAudit{ID: uuid.NewString(), ActorUserID: &actor.ID, Action: "activity.start", EntityType: "activity", EntityID: &activityID, Metadata: metadata, IdempotencyKey: key, CreatedAt: time.Now().UTC()}

	// when
	created, createErr := repository.Create(context.Background(), audit)
	found, findErr := repository.FindByActorAndIdempotencyKey(context.Background(), actor.ID, key)
	_, missingErr := repository.FindByActorAndIdempotencyKey(context.Background(), actor.ID, uuid.NewString())
	duplicate := *audit
	duplicate.ID = uuid.NewString()
	_, duplicateErr := repository.Create(context.Background(), &duplicate)

	// then
	require.NoError(t, createErr)
	require.NoError(t, findErr)
	assert.Equal(t, created.ID, found.ID)
	assert.JSONEq(t, string(metadata), string(found.Metadata))
	assert.ErrorIs(t, missingErr, appErrors.ErrNotFound)
	assert.ErrorIs(t, duplicateErr, appErrors.ErrConflict)
}
