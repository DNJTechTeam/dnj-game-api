package repositories

import (
	"context"
	"encoding/json"
	"time"

	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
	"gorm.io/gorm"
)

func reserveGlobalIdempotencyKey(
	ctx context.Context,
	database *gorm.DB,
	id string,
	actorUserID uint64,
	key string,
	operation string,
	resourceRef *string,
	fingerprint string,
	resultRef *string,
	httpStatus int,
	createdAt time.Time,
) error {
	completedAt := createdAt
	row := &models.IdempotencyOperation{
		ID:               id,
		ActorUserID:      actorUserID,
		IdempotencyKey:   key,
		Operation:        operation,
		ResourceRef:      resourceRef,
		IntentHash:       fingerprint,
		State:            "completed",
		ResultRef:        resultRef,
		ResponseSnapshot: json.RawMessage(`{}`),
		HTTPStatus:       httpStatus,
		CreatedAt:        createdAt,
		CompletedAt:      &completedAt,
	}
	return handleRepositoryError(database.Create(row).Error)
}
