package interfaces

import (
	"context"

	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
)

type ActivityServiceInterface interface {
	Start(ctx context.Context, activityID, idempotencyKey string) (*messages.ActivityStateResponseDTO, error)
	Pause(ctx context.Context, activityID, idempotencyKey string) (*messages.ActivityStateResponseDTO, error)
	Conclude(ctx context.Context, activityID, idempotencyKey string) (*messages.ActivityStateResponseDTO, error)
	StartScheduled(ctx context.Context, activityID, idempotencyKey string) (*messages.ActivityStateResponseDTO, error)
	FlexScheduled(ctx context.Context, activityID, idempotencyKey string) (*messages.ActivityStateResponseDTO, error)
	AdvanceScheduled(ctx context.Context, activityID, idempotencyKey string) (*messages.ActivityStateResponseDTO, error)
}
