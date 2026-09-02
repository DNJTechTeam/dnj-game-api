package repositories

import (
	"context"
	"errors"
	"testing"
	"time"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	activityEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/activity/entities"
	spaceEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/space/entities"
	userEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestAdminInstallation_RepositoryFailuresRemainRedacted(t *testing.T) {
	ctx := context.Background()
	spaceDB, spaceMock := newMockDB(t)
	spaces := NewSpaceRepository(spaceDB)
	spaceMock.ExpectBegin().WillReturnError(errors.New("db down"))
	_, err := spaces.Create(ctx, &spaceEntities.Space{ID: uuid.NewString()})
	assert.ErrorIs(t, err, appErrors.InternalError)
	spaceMock.ExpectQuery(`SELECT`).WillReturnError(errors.New("db down"))
	_, err = spaces.FindByIDForUpdate(ctx, uuid.NewString())
	assert.ErrorIs(t, err, appErrors.InternalError)
	spaceMock.ExpectQuery(`SELECT`).WillReturnError(errors.New("db down"))
	_, err = spaces.List(ctx, 0)
	assert.ErrorIs(t, err, appErrors.InternalError)

	activityDB, activityMock := newMockDB(t)
	activities := NewActivityRepository(activityDB)
	activityMock.ExpectQuery(`SELECT`).WillReturnError(errors.New("db down"))
	_, err = activities.List(ctx, 0)
	assert.ErrorIs(t, err, appErrors.InternalError)
	activityMock.ExpectBegin().WillReturnError(errors.New("db down"))
	_, err = activities.Create(ctx, &activityEntities.Activity{ID: uuid.NewString()})
	assert.ErrorIs(t, err, appErrors.InternalError)
	activityMock.ExpectQuery(`SELECT`).WillReturnError(errors.New("db down"))
	_, err = activities.FindByID(ctx, uuid.NewString())
	assert.ErrorIs(t, err, appErrors.InternalError)

	userDB, userMock := newMockDB(t)
	users := NewUserRepository(userDB)
	userMock.ExpectQuery(`SELECT`).WillReturnError(errors.New("db down"))
	_, err = users.ListByRole(ctx, []userEntities.UserRole{userEntities.RoleDefault}, 0)
	assert.ErrorIs(t, err, appErrors.InternalError)
	userMock.ExpectBegin().WillReturnError(errors.New("db down"))
	err = users.UpdateRole(ctx, 1, userEntities.RoleAdmin)
	assert.ErrorIs(t, err, appErrors.InternalError)

	_ = time.Now()
}
