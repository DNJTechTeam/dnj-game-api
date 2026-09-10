package services

import (
	"testing"

	activityEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/activity/entities"
	gameEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/game/entities"
	"github.com/stretchr/testify/assert"
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
}

func activityIDs(games []activityEntities.PublicActivity) []string {
	ids := make([]string, len(games))
	for index, game := range games {
		ids[index] = game.Activity.ID
	}
	return ids
}
