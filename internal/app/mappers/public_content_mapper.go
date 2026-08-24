package mappers

import (
	"time"

	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	activityEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/activity/entities"
	spaceEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/space/entities"
)

func MapPublicSpaceToResponseDTO(space *spaceEntities.Space) *messages.PublicSpaceResponseDTO {
	if space == nil {
		return nil
	}
	return &messages.PublicSpaceResponseDTO{ID: space.ID, Name: space.Name, Slug: space.Slug}
}

func utcTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	utc := value.UTC()
	return &utc
}

func MapPublicActivityToResponseDTO(item *activityEntities.PublicActivity, state *string) *messages.PublicActivityResponseDTO {
	if item == nil {
		return nil
	}
	activity := item.Activity
	return &messages.PublicActivityResponseDTO{ID: activity.ID, Space: MapPublicSpaceToResponseDTO(item.Space), Slug: activity.Slug, Name: activity.Name, Description: activity.Description, Kind: string(activity.Kind), StartsAt: utcTimePointer(activity.StartsAt), EndsAt: utcTimePointer(activity.EndsAt), CheckInPoints: activity.CheckInPoints, MomentPoints: activity.MomentPoints, CooldownSeconds: activity.CooldownSeconds, AllowsMoment: activity.AllowsMoment, State: state}
}

func MapScheduleItemToResponseDTO(item *activityEntities.PublicActivity, state string) messages.ScheduleItemResponseDTO {
	activity := item.Activity
	return messages.ScheduleItemResponseDTO{ID: activity.ID, Title: activity.Name, Description: activity.Description, StartsAt: activity.StartsAt.UTC(), EndsAt: activity.EndsAt.UTC(), Sector: MapPublicSpaceToResponseDTO(item.Space), State: state}
}
