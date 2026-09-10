package services

import (
	"context"
	"errors"
	"testing"
	"time"

	activityEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/activity/entities"
	gameEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/game/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestFilterManagerGamesByScope(t *testing.T) {
	games := []activityEntities.PublicActivity{
		{Activity: activityEntities.Activity{ID: "radicalidade", Kind: activityEntities.KindCompetitive}},
		{Activity: activityEntities.Activity{ID: "evento-especial", Kind: activityEntities.KindLive}},
		{Activity: activityEntities.Activity{ID: "desafio", Kind: activityEntities.KindChallenge}},
	}

	assert.Equal(t, []string{"radicalidade"}, activityIDs(filterManagerGamesByScope(games, "actions", false)))
	assert.Equal(t, []string{"evento-especial"}, activityIDs(filterManagerGamesByScope(games, "special_events", false)))
	assert.Empty(t, filterManagerGamesByScope(games, "space", false))
	assert.Empty(t, filterManagerGamesByScope(games, "pastoral_queue", false))
	assert.Len(t, filterManagerGamesByScope(games, "actions", true), 3)
}

func TestManagerRunVisibleForScope(t *testing.T) {
	competitive := &gameEntities.ActivityRun{Activity: &activityEntities.Activity{Kind: activityEntities.KindCompetitive}}
	checkpoint := &gameEntities.ActivityRun{Activity: &activityEntities.Activity{Kind: activityEntities.KindCheckpoint}}
	live := &gameEntities.ActivityRun{Activity: &activityEntities.Activity{Kind: activityEntities.KindLive}}

	assert.True(t, managerRunVisibleForScope(competitive, "actions", false))
	assert.False(t, managerRunVisibleForScope(checkpoint, "actions", false))
	assert.False(t, managerRunVisibleForScope(live, "actions", false))
	assert.True(t, managerRunVisibleForScope(live, "special_events", false))
	assert.True(t, managerRunVisibleForScope(checkpoint, "anything", true))
	assert.False(t, managerRunVisibleForScope(nil, "actions", false))
	assert.True(t, managerRunVisibleForScope(&gameEntities.ActivityRun{}, "actions", false))
	assert.False(t, managerRunVisibleForScope(competitive, "unknown", false))
}

func TestManagerDashboardRunDTOIncludesActivityAndQR(t *testing.T) {
	games := mocks.NewMockGameRepositoryInterface(t)
	service := &GameService{games: games, secret: func() string { return "test-secret" }}
	expiresAt := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	games.EXPECT().FindActiveQRByRun(mock.Anything, "run-1").Return(&gameEntities.QRCode{ID: "qr-1", ExpiresAt: expiresAt}, nil)

	run := &gameEntities.ActivityRun{
		ID:         "run-1",
		ActivityID: "activity-1",
		Status:     gameEntities.RunStatusDraft,
		Activity: &activityEntities.Activity{
			ID:   "activity-1",
			Name: "Corrida do saco",
		},
	}
	participants := []gameEntities.RunParticipant{{ID: "participant-1", Name: "Ana"}}

	dto := service.managerDashboardRunDTO(context.Background(), run, participants)

	require.NotNil(t, dto)
	assert.Equal(t, "run-1", dto.ID)
	assert.Equal(t, "activity-1", dto.GameID)
	assert.Equal(t, "Corrida do saco", dto.GameName)
	assert.Equal(t, "checkin", dto.Status)
	assert.NotEmpty(t, dto.QRToken)
	require.NotNil(t, dto.QRExpiresAt)
	assert.Equal(t, expiresAt, *dto.QRExpiresAt)
	require.Len(t, dto.Participants, 1)
	assert.Equal(t, "Ana", dto.Participants[0].Name)
}

func TestManagerDashboardRunDTOHandlesMissingActivityAndQRLookupError(t *testing.T) {
	games := mocks.NewMockGameRepositoryInterface(t)
	service := &GameService{games: games, secret: func() string { return "test-secret" }}

	withoutActivity := service.managerDashboardRunDTO(context.Background(), &gameEntities.ActivityRun{ID: "run-1"}, nil)
	require.NotNil(t, withoutActivity)
	assert.Empty(t, withoutActivity.GameName)
	assert.Empty(t, withoutActivity.QRToken)

	games.EXPECT().FindActiveQRByRun(mock.Anything, "run-2").Return(nil, errors.New("qr unavailable"))
	withQRFailure := service.managerDashboardRunDTO(context.Background(), &gameEntities.ActivityRun{
		ID:       "run-2",
		Activity: &activityEntities.Activity{ID: "activity-2", Name: "Outro jogo"},
	}, nil)
	assert.Empty(t, withQRFailure.QRToken)
	assert.Nil(t, withQRFailure.QRExpiresAt)
}

func activityIDs(games []activityEntities.PublicActivity) []string {
	ids := make([]string, len(games))
	for index, game := range games {
		ids[index] = game.Activity.ID
	}
	return ids
}
