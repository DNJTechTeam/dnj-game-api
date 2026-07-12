package interfaces

import (
	"context"

	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
)

type UserServiceInterface interface {
	UpdateGroup(ctx context.Context, userID uint64, request *messages.UpdateUserGroupRequestDTO) (*messages.UserResponseDTO, error)
}
