package interfaces

import (
	"context"

	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
)

type IdentityServiceInterface interface {
	AuthenticateGoogle(ctx context.Context, request *messages.GoogleAuthRequestDTO) (*messages.IdentitySessionResponseDTO, error)
	Refresh(ctx context.Context, refreshToken string) (*messages.IdentitySessionResponseDTO, error)
	Current(ctx context.Context) (*messages.CurrentSessionResponseDTO, error)
	CompleteOnboarding(ctx context.Context, request *messages.CompleteOnboardingRequestDTO) (*messages.CurrentSessionResponseDTO, error)
	Logout(ctx context.Context, refreshToken string) error
}
