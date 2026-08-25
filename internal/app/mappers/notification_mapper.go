package mappers

import (
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	"github.com/dnjtechteam/dnj-game-api/internal/domain/notification/entities"
)

func MapNotificationToResponseDTO(item *entities.Notification) *messages.NotificationResponseDTO {
	if item == nil {
		return nil
	}
	return &messages.NotificationResponseDTO{
		ID:         item.ID,
		Category:   string(item.Category),
		State:      string(item.State),
		Title:      item.Title,
		Body:       item.Body,
		SourceType: item.SourceType,
		SourceID:   item.SourceID,
		CreatedAt:  item.CreatedAt.UTC(),
		ReadAt:     item.ReadAt,
	}
}

func MapNotificationsToResponseDTOs(items []entities.Notification) []messages.NotificationResponseDTO {
	dtos := make([]messages.NotificationResponseDTO, len(items))
	for i := range items {
		dtos[i] = *MapNotificationToResponseDTO(&items[i])
	}
	return dtos
}

func MapNotificationPreferencesToResponseDTO(item *entities.Preferences) *messages.NotificationPreferencesResponseDTO {
	if item == nil {
		return nil
	}
	return &messages.NotificationPreferencesResponseDTO{
		MomentModerationEnabled: true,
		PointsEnabled:           item.PointsEnabled,
		AnnouncementEnabled:     item.AnnouncementEnabled,
		UpdatedAt:               item.UpdatedAt.UTC(),
	}
}
