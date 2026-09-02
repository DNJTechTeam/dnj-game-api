package mappers

import (
	"github.com/dnjtechteam/dnj-game-api/internal/domain/notification/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
)

func MapNotificationToEntity(row *models.Notification) *entities.Notification {
	if row == nil {
		return nil
	}
	return &entities.Notification{
		ID:         row.ID,
		UserID:     row.UserID,
		Category:   entities.Category(row.Category),
		State:      entities.State(row.State),
		Title:      row.Title,
		Body:       row.Body,
		SourceType: row.SourceType,
		SourceID:   row.SourceID,
		Metadata:   row.Metadata,
		CreatedAt:  row.CreatedAt,
		ReadAt:     row.ReadAt,
	}
}

func MapNotificationEntityToModel(item *entities.Notification) *models.Notification {
	if item == nil {
		return nil
	}
	return &models.Notification{
		ID:         item.ID,
		UserID:     item.UserID,
		Category:   string(item.Category),
		State:      string(item.State),
		Title:      item.Title,
		Body:       item.Body,
		SourceType: item.SourceType,
		SourceID:   item.SourceID,
		Metadata:   item.Metadata,
		CreatedAt:  item.CreatedAt,
		ReadAt:     item.ReadAt,
	}
}

func MapNotificationPreferenceToEntity(row *models.NotificationPreference) *entities.Preferences {
	if row == nil {
		return nil
	}
	return &entities.Preferences{
		UserID:              row.UserID,
		PointsEnabled:       row.PointsEnabled,
		AnnouncementEnabled: row.AnnouncementEnabled,
		UpdatedAt:           row.UpdatedAt,
	}
}

func MapNotificationOperationToEntity(row *models.IdempotencyOperation) *entities.Operation {
	if row == nil {
		return nil
	}
	return &entities.Operation{
		ID:               row.ID,
		ActorUserID:      row.ActorUserID,
		IdempotencyKey:   row.IdempotencyKey,
		Operation:        row.Operation,
		ResourceRef:      row.ResourceRef,
		IntentHash:       row.IntentHash,
		State:            row.State,
		ResultRef:        row.ResultRef,
		ResultCount:      row.ResultCount,
		ResponseSnapshot: row.ResponseSnapshot,
		HTTPStatus:       row.HTTPStatus,
		CreatedAt:        row.CreatedAt,
		CompletedAt:      row.CompletedAt,
	}
}
