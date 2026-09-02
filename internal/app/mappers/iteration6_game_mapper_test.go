package mappers

import (
	"testing"
	"time"

	activityEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/activity/entities"
	gameEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/game/entities"
	spaceEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/space/entities"
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

func TestIteration6GameMapper_GameAndRankingProjection(t *testing.T) {
	startsAt := time.Date(2026, 8, 24, 18, 0, 0, 0, time.FixedZone("offset", -3*60*60))
	endsAt := startsAt.Add(time.Hour)
	description := "Description"
	state := "live"
	groupName := "Equipe"
	game := MapGameToResponseDTO(&activityEntities.PublicActivity{
		Activity: activityEntities.Activity{ID: "game", Slug: "game-slug", Name: "Game", Description: &description, StartsAt: &startsAt, EndsAt: &endsAt, AllowsMoment: true},
		Space:    &spaceEntities.Space{ID: "space", Name: "Arena", Slug: "arena"},
	}, &state)
	assert.Equal(t, "game", game.ID)
	assert.Equal(t, "Arena", game.Space.Name)
	assert.Equal(t, time.UTC, game.StartsAt.Location())
	assert.Equal(t, "live", *game.State)

	individual := MapIndividualRankingToResponseDTO(gameEntities.IndividualRanking{UserID: 42, Name: "Ana", GroupName: &groupName, Points: 10, Position: 1})
	group := MapGroupRankingToResponseDTO(gameEntities.GroupRanking{GroupID: 7, Name: "Equipe", Members: 3, Points: 20, Position: 1})
	assert.Equal(t, uint64(42), uint64(individual.ID))
	assert.Equal(t, "Equipe", *individual.GroupName)
	assert.Equal(t, uint64(7), uint64(group.ID))
	assert.Equal(t, 3, group.Members)

	assert.Equal(t, "Momento em desafio", MapPointEntryToResponseDTO(gameEntities.PointEntry{Reason: "moment_challenge_award"}).Label)
	assert.Equal(t, "Ajuste de Momento - Chafariz", MapPointEntryToResponseDTO(gameEntities.PointEntry{Reason: "moment_moderation_reversal", ActivityName: "Chafariz"}).Label)
	assert.Equal(t, "activity_run_first", PointReason(gameEntities.ResultFirst))
}
