package interfaces

import (
	"context"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
)

type ProfileServiceInterface interface {
	Current(ctx context.Context) (*messages.CurrentProfileResponseDTO, error)
	Update(ctx context.Context, request *messages.UpdateCurrentProfileRequestDTO) (*messages.CurrentProfileResponseDTO, error)
}
