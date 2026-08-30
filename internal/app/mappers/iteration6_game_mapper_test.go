package mappers

import (
	"testing"
	"time"

	gameEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/game/entities"
	"github.com/stretchr/testify/assert"
)

func TestIteration6GameMapper_PointPresentationIsAllowlisted(t *testing.T) {
	now := time.Date(2026, 8, 24, 18, 0, 0, 0, time.FixedZone("negative", -3*60*60))
	tests := []struct {
		reason string
		label  string
		icon   string
	}{
		{"activity_run_first", "1º lugar em jogo", "trophy"},
		{"activity_run_second", "2º lugar em jogo", "medal"},
		{"activity_run_third", "3º lugar em jogo", "medal"},
		{"activity_run_participation", "Participação em jogo", "game"},
		{"internal_unknown", "Pontos DNJ", "points"},
	}
	for _, testCase := range tests {
		response := MapPointEntryToResponseDTO(gameEntities.PointEntry{ID: "entry", Reason: testCase.reason, Delta: 10, CreatedAt: now})
		assert.Equal(t, testCase.label, response.Label)
		assert.Equal(t, testCase.icon, response.Icon)
		assert.Equal(t, time.UTC, response.CreatedAt.Location())
	}
	response := MapPointEntryToResponseDTO(gameEntities.PointEntry{ID: "entry", Reason: "activity_run_first", ActivityName: "Corrida do Saco", Delta: 10, CreatedAt: now})
	assert.Equal(t, "1º lugar em Corrida do Saco", response.Label)
	response = MapPointEntryToResponseDTO(gameEntities.PointEntry{ID: "entry", Reason: "moment_challenge_award", ActivityName: "Chafariz", Delta: 10, CreatedAt: now})
	assert.Equal(t, "Desafio Momento - Chafariz", response.Label)
}

func TestIteration6GameMapper_OptionalParticipationAndResultFields(t *testing.T) {
	result := gameEntities.ResultFirst
	runParticipant := MapRunParticipantToResponseDTO(gameEntities.RunParticipant{ID: "participant", Result: &result})
	assert.Equal(t, "first", *runParticipant.Result)

	spaceID := "space"
	spaceName := "Arena"
	withPlace := MapParticipationToResponseDTO(&gameEntities.Participation{ID: "participation", SpaceID: &spaceID, SpaceName: &spaceName}, nil)
	withoutPlace := MapParticipationToResponseDTO(&gameEntities.Participation{ID: "participation"}, nil)
	assert.Equal(t, spaceID, withPlace.Place.ID)
	assert.Nil(t, withoutPlace.Place)
}
