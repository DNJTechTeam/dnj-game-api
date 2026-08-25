package interfaces

import (
	"context"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
)

type MediaServiceInterface interface {
	CreateUploadIntent(context.Context, string, *messages.CreateUploadIntentRequestDTO) (*messages.UploadIntentResponseDTO, int, error)
	CompleteUpload(context.Context, string, string) (*messages.MediaAssetResponseDTO, int, error)
}
type MomentServiceInterface interface {
	List(context.Context, string, string) (*messages.MomentPageResponseDTO, error)
	Create(context.Context, string, *messages.CreateMomentRequestDTO) (*messages.MomentResponseDTO, int, error)
	ToggleLike(context.Context, string, string) (*messages.LikeResponseDTO, error)
	ListModeration(context.Context, string, uint64) (*messages.ModerationPageResponseDTO, error)
	Moderate(context.Context, string, string, *messages.ModerationRequestDTO) (*messages.ModerationResponseDTO, error)
}
