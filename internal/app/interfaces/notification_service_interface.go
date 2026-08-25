package interfaces

import (
	"context"

	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
)

type NotificationServiceInterface interface {
	GetPreferences(ctx context.Context) (*messages.NotificationPreferencesResponseDTO, error)
	UpdatePreferences(
		ctx context.Context,
		rawKey string,
		request *messages.UpdateNotificationPreferencesRequestDTO,
	) (*messages.NotificationPreferencesResponseDTO, error)
	List(ctx context.Context, filter *messages.ListNotificationsFilterDTO) (*messages.NotificationListResponseDTO, error)
	MarkRead(ctx context.Context, rawNotificationID string, rawKey string) (*messages.NotificationResponseDTO, error)
	AdminSend(
		ctx context.Context,
		rawKey string,
		request *messages.AdminSendNotificationRequestDTO,
	) (*messages.AdminSendNotificationResponseDTO, error)
}
