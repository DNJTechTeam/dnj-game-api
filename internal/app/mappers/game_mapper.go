package mappers

import (
	"fmt"
	"strings"

	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	activityEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/activity/entities"
	gameEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/game/entities"
)

func MapGameToResponseDTO(game *activityEntities.PublicActivity, state *string) messages.GameResponseDTO {
	activity := game.Activity
	return messages.GameResponseDTO{ID: activity.ID, Space: MapPublicSpaceToResponseDTO(game.Space), Slug: activity.Slug, Name: activity.Name, Description: activity.Description, StartsAt: utcTimePointer(activity.StartsAt), EndsAt: utcTimePointer(activity.EndsAt), AllowsMoment: activity.AllowsMoment, State: state}
}

func MapIndividualRankingToResponseDTO(item gameEntities.IndividualRanking) messages.IndividualRankingResponseDTO {
	return messages.IndividualRankingResponseDTO{ID: messages.Uint64StringFromUint64(item.UserID), Name: item.Name, GroupName: item.GroupName, Points: item.Points, Position: item.Position}
}

func MapGroupRankingToResponseDTO(item gameEntities.GroupRanking) messages.GroupRankingResponseDTO {
	return messages.GroupRankingResponseDTO{ID: messages.Uint64StringFromUint64(item.GroupID), Name: item.Name, Members: item.Members, Points: item.Points, Position: item.Position}
}

func pointPresentation(reason, activityName string) (string, string) {
	activityName = strings.TrimSpace(activityName)
	withActivity := func(label, fallback string) string {
		if activityName == "" {
			return fallback
		}
		return label + " em " + activityName
	}
	switch reason {
	case "activity_run_first":
		return withActivity("1º lugar", "1º lugar em jogo"), "trophy"
	case "activity_run_second":
		return withActivity("2º lugar", "2º lugar em jogo"), "medal"
	case "activity_run_third":
		return withActivity("3º lugar", "3º lugar em jogo"), "medal"
	case "activity_run_participation":
		return withActivity("Participação", "Participação em jogo"), "game"
	case "moment_challenge_award":
		if activityName != "" {
			return "Desafio Momento - " + activityName, "camera"
		}
		return "Momento em desafio", "camera"
	case "moment_moderation_reversal":
		if activityName != "" {
			return "Ajuste de Momento - " + activityName, "shield"
		}
		return "Ajuste de Momento", "shield"
	default:
		return "Pontos DNJ", "points"
	}
}

func MapPointEntryToResponseDTO(item gameEntities.PointEntry) messages.PointEntryResponseDTO {
	label, icon := pointPresentation(item.Reason, item.ActivityName)
	return messages.PointEntryResponseDTO{ID: item.ID, Label: label, Points: item.Delta, Icon: icon, CreatedAt: item.CreatedAt.UTC()}
}

func MapRunParticipantToResponseDTO(item gameEntities.RunParticipant) messages.RunParticipantResponseDTO {
	var result *string
	if item.Result != nil {
		value := string(*item.Result)
		result = &value
	}
	return messages.RunParticipantResponseDTO{ID: item.ID, Name: item.Name, CheckedInAt: item.CheckedInAt.UTC(), Result: result, PointsAwarded: item.PointsAwarded}
}

func MapParticipationToResponseDTO(item *gameEntities.Participation, total *int) messages.ParticipationResponseDTO {
	var place *messages.NamedGameReferenceDTO
	if item.SpaceID != nil && item.SpaceName != nil {
		place = &messages.NamedGameReferenceDTO{ID: *item.SpaceID, Name: *item.SpaceName}
	}
	return messages.ParticipationResponseDTO{ID: item.ID, Activity: messages.NamedGameReferenceDTO{ID: item.ActivityID, Name: item.ActivityName}, Place: place, CheckedInAt: item.CheckedInAt.UTC(), Status: string(item.Status), CanShareMoment: item.CanShareMoment, CheckInPoints: item.CheckInPoints, NewTotalPoints: total}
}

func PointReason(result gameEntities.Result) string {
	return fmt.Sprintf("activity_run_%s", result)
}
