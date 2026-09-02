package interfaces

import (
	"context"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
)

type SpecialEventServiceInterface interface {
	ListManager(ctx context.Context) (*messages.ManagerSpecialEventsResponseDTO, error)
	Create(ctx context.Context, request *messages.CreateSpecialEventRequestDTO) (*messages.ManagerSpecialEventDTO, error)
	Teaser(ctx context.Context, eventID string) (*messages.ManagerSpecialEventDTO, error)
	ReleaseQR(ctx context.Context, eventID string) (*messages.SpecialEventQRResponseDTO, error)
	Close(ctx context.Context, eventID string) error
	Active(ctx context.Context, target string) (*messages.ActiveSpecialEventResponseDTO, error)
	Display(ctx context.Context, target string) (*messages.LiveDisplaySpecialEventDTO, error)
}
