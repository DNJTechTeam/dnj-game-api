package services

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	mediaEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/media/entities"
	momentEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/moment/entities"
	auditEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/operationaudit/entities"
	"github.com/google/uuid"
)

func (s *MomentService) ToggleLike(
	ctx context.Context,
	rawMomentID string,
	rawKey string,
) (*messages.LikeResponseDTO, error) {
	momentID, err := uuid.Parse(rawMomentID)
	if err != nil {
		return nil, notFoundError()
	}
	key, err := parseIdempotencyKey(rawKey)
	if err != nil {
		return nil, err
	}
	actor, err := requireDefaultActor(ctx, s.users, false)
	if err != nil {
		return nil, err
	}

	operation := "moment.like.toggle"
	fingerprint := intentHash(operation, struct {
		MomentID string `json:"momentId"`
	}{MomentID: momentID.String()})
	now := utcNow(s.now)
	var response *messages.LikeResponseDTO

	err = s.WithTransaction(ctx, func(tx context.Context) error {
		moment, findErr := s.moments.FindMoment(tx, momentID.String(), actor.ID, true)
		if findErr != nil {
			return notFoundError()
		}
		prior, priorErr := findIdempotencyOperation(
			tx,
			s.media,
			actor.ID,
			key,
			operation,
			fingerprint,
		)
		if priorErr != nil {
			return priorErr
		}
		if prior != nil {
			if prior.ResultBoolean == nil || prior.ResultCount == nil {
				return appErrors.InternalError
			}
			response = &messages.LikeResponseDTO{
				MomentID:   momentID.String(),
				Liked:      *prior.ResultBoolean,
				LikesCount: *prior.ResultCount,
			}
			return nil
		}
		if _, authErr := requireDefaultActor(tx, s.users, true); authErr != nil {
			return authErr
		}
		visible := moment.PublicationStatus == momentEntities.PublicationPublic &&
			moment.ModerationStatus == momentEntities.ModerationApproved &&
			moment.AssetAvailable &&
			moment.AuthorEligible &&
			now.Before(moment.AssetRetentionDueAt)
		if !visible {
			return notFoundError()
		}

		liked, count, toggleErr := s.moments.ToggleLike(tx, moment.ID, actor.ID, now)
		if toggleErr != nil {
			return appErrors.InternalError
		}
		completedAt := now
		if createErr := createIdempotencyOperation(tx, s.media, &mediaEntities.Operation{
			ID:               uuid.NewString(),
			ActorUserID:      actor.ID,
			IdempotencyKey:   key,
			Operation:        operation,
			ResourceRef:      &moment.ID,
			IntentHash:       fingerprint,
			State:            "completed",
			ResultRef:        &moment.ID,
			ResultBoolean:    &liked,
			ResultCount:      &count,
			ResponseSnapshot: []byte(`{}`),
			HTTPStatus:       http.StatusOK,
			CreatedAt:        now,
			CompletedAt:      &completedAt,
		}); createErr != nil {
			return appErrors.InternalError
		}
		response = &messages.LikeResponseDTO{
			MomentID:   moment.ID,
			Liked:      liked,
			LikesCount: count,
		}
		return nil
	})
	if errors.Is(err, errIdempotencyRace) {
		return s.ToggleLike(ctx, rawMomentID, rawKey)
	}
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (s *MomentService) ListModeration(
	ctx context.Context,
	queue string,
	page uint64,
) (*messages.ModerationPageResponseDTO, error) {
	if queue != "general" && queue != "challenge" {
		return nil, mediaMomentError(
			http.StatusBadRequest,
			"INVALID_REQUEST",
			"queue deve ser general ou challenge.",
		)
	}
	if _, err := requireAdminActor(ctx, s.users); err != nil {
		return nil, err
	}
	now := utcNow(s.now)
	pageResult, err := s.moments.ListModeration(ctx, queue, page, now)
	if err != nil {
		return nil, appErrors.InternalError
	}
	response := &messages.ModerationPageResponseDTO{
		Data: make([]messages.ModerationMomentResponseDTO, len(pageResult.Items)),
		Pagination: messages.ModerationPaginationDTO{
			CurrentPage: messages.Uint64StringFromUint64(page + 1),
			HasNextPage: pageResult.HasNext,
			Limit:       50,
		},
	}
	for index := range pageResult.Items {
		item, mapErr := s.moderationQueueItem(ctx, &pageResult.Items[index], now)
		if mapErr != nil {
			return nil, mapErr
		}
		response.Data[index] = *item
	}
	return response, nil
}

func (s *MomentService) moderationQueueItem(
	ctx context.Context,
	moment *momentEntities.Moment,
	signingTime time.Time,
) (*messages.ModerationMomentResponseDTO, error) {
	asset, err := s.media.FindAsset(ctx, moment.MediaAssetID, false)
	if err != nil {
		return nil, appErrors.InternalError
	}
	signedURL, err := s.storage.PresignDownload(ctx, asset, signingTime, mediaReadLifetime)
	if err != nil {
		return nil, mediaUnavailableError()
	}
	expiresAt := signingTime.Add(mediaReadLifetime).UTC()
	actions := []string{"delete_photo"}
	if moment.RewardStatus == momentEntities.RewardAwarded {
		actions = []string{"deny_points", "delete_photo"}
	}
	var activity *messages.ModerationActivitySummaryDTO
	if moment.ActivityID != nil {
		activity = &messages.ModerationActivitySummaryDTO{
			ID:   *moment.ActivityID,
			Name: stringValue(moment.ActivityName),
		}
	}
	return &messages.ModerationMomentResponseDTO{
		MomentID:          moment.ID,
		ImageURL:          signedURL,
		ImageExpiresAt:    &expiresAt,
		CapturedAt:        moment.CapturedAt.UTC(),
		ParticipantName:   moment.AuthorName,
		Activity:          activity,
		PointsAwarded:     moment.PointsAwarded,
		PublicationStatus: string(moment.PublicationStatus),
		ModerationStatus:  string(moment.ModerationStatus),
		RewardStatus:      string(moment.RewardStatus),
		PhotoStatus:       "available",
		AvailableActions:  actions,
	}, nil
}

func (s *MomentService) Moderate(
	ctx context.Context,
	rawMomentID string,
	rawKey string,
	request *messages.ModerationRequestDTO,
) (*messages.ModerationResponseDTO, error) {
	momentID, err := uuid.Parse(rawMomentID)
	if err != nil {
		return nil, notFoundError()
	}
	if request == nil || request.Action != "deny_points" && request.Action != "delete_photo" {
		return nil, mediaMomentError(
			http.StatusBadRequest,
			"INVALID_REQUEST",
			"action deve ser deny_points ou delete_photo.",
		)
	}
	key, err := parseIdempotencyKey(rawKey)
	if err != nil {
		return nil, err
	}
	actor, err := requireAdminActor(ctx, s.users)
	if err != nil {
		return nil, err
	}
	operation := "admin.moment.moderate"
	fingerprint := intentHash(operation, struct {
		MomentID string `json:"momentId"`
		Action   string `json:"action"`
	}{MomentID: momentID.String(), Action: request.Action})
	now := utcNow(s.now)
	var response *messages.ModerationResponseDTO

	err = s.WithTransaction(ctx, func(tx context.Context) error {
		prior, priorErr := findIdempotencyOperation(
			tx,
			s.media,
			actor.ID,
			key,
			operation,
			fingerprint,
		)
		if priorErr != nil {
			return priorErr
		}
		if prior != nil {
			return s.replayModeration(tx, momentID.String(), request.Action, actor.ID, &response)
		}
		if _, authErr := requireAdminActor(tx, s.users); authErr != nil {
			return authErr
		}

		moment, asset, changed, applyErr := s.moments.ApplyModeration(
			tx,
			momentID.String(),
			request.Action,
			actor.ID,
			key,
			now,
		)
		if errors.Is(applyErr, appErrors.ErrNotFound) {
			return notFoundError()
		}
		if errors.Is(applyErr, appErrors.ErrConflict) {
			return mediaMomentError(
				http.StatusConflict,
				"MODERATION_ACTION_INVALID",
				"A ação não é válida para este Moment.",
			)
		}
		if applyErr != nil {
			return appErrors.InternalError
		}
		if changed {
			if effectErr := s.recordModerationEffect(
				tx,
				moment,
				asset,
				actor.ID,
				key,
				request.Action,
				now,
			); effectErr != nil {
				return effectErr
			}
		}

		completedAt := now
		if createErr := createIdempotencyOperation(tx, s.media, &mediaEntities.Operation{
			ID:               uuid.NewString(),
			ActorUserID:      actor.ID,
			IdempotencyKey:   key,
			Operation:        operation,
			ResourceRef:      &moment.ID,
			IntentHash:       fingerprint,
			State:            "completed",
			ResultRef:        &moment.ID,
			ResponseSnapshot: []byte(`{}`),
			HTTPStatus:       http.StatusOK,
			CreatedAt:        now,
			CompletedAt:      &completedAt,
		}); createErr != nil {
			return appErrors.InternalError
		}
		response = moderationResult(moment, asset, request.Action)
		return nil
	})
	if errors.Is(err, errIdempotencyRace) {
		return s.Moderate(ctx, rawMomentID, rawKey, request)
	}
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (s *MomentService) replayModeration(
	ctx context.Context,
	momentID string,
	action string,
	actorID uint64,
	response **messages.ModerationResponseDTO,
) error {
	moment, err := s.moments.FindMoment(ctx, momentID, actorID, false)
	if err != nil {
		return appErrors.InternalError
	}
	asset, err := s.media.FindAsset(ctx, moment.MediaAssetID, false)
	if err != nil {
		return appErrors.InternalError
	}
	*response = moderationResult(moment, asset, action)
	return nil
}

func (s *MomentService) recordModerationEffect(
	ctx context.Context,
	moment *momentEntities.Moment,
	asset *mediaEntities.Asset,
	actorID uint64,
	key string,
	action string,
	now time.Time,
) error {
	created, err := s.moments.CreateModerationDecision(ctx, &momentEntities.ModerationDecision{
		ID:             uuid.NewString(),
		MomentID:       moment.ID,
		ActorUserID:    actorID,
		Action:         action,
		IdempotencyKey: key,
		CreatedAt:      now,
	})
	if err != nil || !created {
		return appErrors.InternalError
	}
	metadata, _ := json.Marshal(map[string]string{"action": action})
	entityID := moment.ID
	_, err = s.audits.Create(ctx, &auditEntities.OperationAudit{
		ID:              uuid.NewString(),
		ActorUserID:     &actorID,
		Action:          "moment.moderated",
		EntityType:      "moment",
		EntityID:        &entityID,
		EntityReference: &entityID,
		Metadata:        metadata,
		IdempotencyKey:  key,
		CreatedAt:       now,
	})
	if err != nil {
		return appErrors.InternalError
	}
	if action == "delete_photo" {
		_, err = s.media.CreateCleanupJob(ctx, &mediaEntities.CleanupJob{
			ID:            uuid.NewString(),
			MediaAssetID:  asset.ID,
			Kind:          "delete_photo",
			State:         "pending",
			DueAt:         now,
			MaxAttempts:   8,
			NextAttemptAt: now,
			CreatedAt:     now,
			UpdatedAt:     now,
		})
		if err != nil {
			return appErrors.InternalError
		}
	}
	return nil
}

func moderationResult(
	moment *momentEntities.Moment,
	asset *mediaEntities.Asset,
	action string,
) *messages.ModerationResponseDTO {
	photoStatus := "available"
	if asset.State == mediaEntities.AssetDeleted {
		photoStatus = "deleted"
	}
	return &messages.ModerationResponseDTO{
		MomentID:          moment.ID,
		Action:            action,
		PublicationStatus: string(moment.PublicationStatus),
		ModerationStatus:  string(moment.ModerationStatus),
		RewardStatus:      string(moment.RewardStatus),
		PhotoStatus:       photoStatus,
		PointsAwarded:     moment.PointsAwarded,
	}
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
