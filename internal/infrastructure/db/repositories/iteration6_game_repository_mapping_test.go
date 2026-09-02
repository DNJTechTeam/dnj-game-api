package repositories

import (
	"testing"
	"time"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIteration6GameRepository_MappingAndQueryBranches(t *testing.T) {
	_, err := mapRunModel(nil)
	assert.ErrorIs(t, err, appErrors.ErrNotFound)
	_, err = mapRunModel(&models.ActivityRun{PointRules: []byte("not-json")})
	assert.ErrorIs(t, err, appErrors.InternalError)

	startedAt := time.Date(2026, 8, 24, 18, 0, 0, 0, time.UTC)
	run, err := mapRunModel(&models.ActivityRun{ID: "run", ActivityID: "game", StartedBy: 42, Status: "active", PointRules: []byte(`{"first":50,"second":30,"third":20,"participation":10}`), StartedAt: &startedAt})
	require.NoError(t, err)
	assert.Equal(t, "run", run.ID)
	assert.Equal(t, 50, run.PointRules.First)
	model, err := mapRunEntity(run)
	require.NoError(t, err)
	assert.Equal(t, run.ID, model.ID)

	operation := mapManagerOperation(&models.ManagerOperation{ID: "operation", ActorUserID: 42, Operation: "manager.game.create", ActivityID: "game", HTTPStatus: 201})
	assert.Equal(t, uint64(42), operation.ActorUserID)
	assert.Equal(t, "manager.game.create", operation.Operation)

	participation := mapParticipationRow(&participationRow{Participation: models.Participation{ID: "participation", UserID: 42, ActivityID: "game", ActivityRunID: "run", Status: "active", CheckInPoints: 5}, ActivityName: "Game"})
	assert.Equal(t, "Game", participation.ActivityName)
	assert.Equal(t, uint64(42), participation.UserID)

	db, _ := newMockDB(t)
	assert.NotNil(t, managerRunQuery(db, 42, true))
	assert.NotNil(t, managerRunQuery(db, 42, false))
	assert.NotNil(t, participationProjection(db))
}
