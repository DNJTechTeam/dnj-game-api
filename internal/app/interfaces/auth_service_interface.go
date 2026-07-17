package interfaces

import (
	"context"

	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
)

type AuthServiceInterface interface {
	Onboarding(ctx context.Context, request *messages.OnboardingRequestDTO) (*messages.OnboardingResponseDTO, error)
	VerifyCode(ctx context.Context, request *messages.VerificationCodeRequestDTO) (*messages.VerificationCodeResponseDTO, error)
}
