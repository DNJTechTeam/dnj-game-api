package services

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	appInterfaces "github.com/dnjtechteam/dnj-game-api/internal/app/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	eventInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/eventsettings/interfaces"
	gameEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/game/entities"
	mediaEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/media/entities"
	mediaInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/media/interfaces"
	momentEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/moment/entities"
	momentInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/moment/interfaces"
	auditInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/operationaudit/interfaces"
	userEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
	userInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/user/interfaces"
	"github.com/google/uuid"
)

const mediaReadLifetime = 5 * time.Minute

// freeMomentPoints is the flat reward for sharing a spontaneous Moment with
// the feed (publishConsent=true) while scoring is open.
const freeMomentPoints = 20

type MomentService struct {
	*BaseService
	moments       momentInterfaces.Repository
	media         mediaInterfaces.Repository
	storage       mediaInterfaces.Storage
	users         userInterfaces.UserRepositoryInterface
	audits        auditInterfaces.OperationAuditRepositoryInterface
	eventSettings eventInterfaces.EventSettingsRepositoryInterface
	now           func() time.Time
	cursorSecret  func() string
}

type activeMomentChallengeRepository interface {
	FindActiveMomentChallengeForUpdate(context.Context, time.Time) (string, int, error)
	HasMomentForActivity(context.Context, uint64, string) (bool, error)
}

func NewMomentService(
	base *BaseService,
	moments momentInterfaces.Repository,
	media mediaInterfaces.Repository,
	storage mediaInterfaces.Storage,
	users userInterfaces.UserRepositoryInterface,
	audits auditInterfaces.OperationAuditRepositoryInterface,
	eventSettings eventInterfaces.EventSettingsRepositoryInterface,
) appInterfaces.MomentServiceInterface {
	return &MomentService{
		BaseService:   base,
		moments:       moments,
		media:         media,
		storage:       storage,
		users:         users,
		audits:        audits,
		eventSettings: eventSettings,
		now:           time.Now,
		cursorSecret:  func() string { return os.Getenv("DNJ_CURSOR_HMAC_SECRET") },
	}
}

func moderationMessage(moment *momentEntities.Moment) *string {
	if moment.ModerationStatus != momentEntities.ModerationRejected {
		return nil
	}
	message := "Sua foto não atendeu às regras de publicação."
	if !moment.AssetAvailable {
		message = "Sua foto foi removida da galeria."
	}
	return &message
}

func (s *MomentService) responseFor(
	ctx context.Context,
	moment *momentEntities.Moment,
	signingTime time.Time,
) (*messages.MomentResponseDTO, error) {
	response := &messages.MomentResponseDTO{
		ID:                 moment.ID,
		Origin:             string(moment.Origin),
		ParticipationID:    moment.ParticipationID,
		PlaceName:          moment.PlaceName,
		AuthorName:         moment.AuthorName,
		AuthorAvatarURL:    moment.AuthorAvatarURL,
		CapturedAt:         moment.CapturedAt.UTC(),
		PublicationStatus:  string(moment.PublicationStatus),
		ModerationStatus:   string(moment.ModerationStatus),
		PointsAwarded:      moment.PointsAwarded,
		ModerationMessage:  moderationMessage(moment),
		LikesCount:         moment.LikesCount,
		LikedByCurrentUser: moment.LikedByCurrentUser,
	}
	if moment.GroupID != nil {
		groupID := messages.Uint64StringFromUint64(*moment.GroupID)
		response.GroupID = &groupID
	}
	if moment.GroupName != nil {
		response.GroupName = moment.GroupName
	}

	if !moment.AssetAvailable || !signingTime.Before(moment.AssetRetentionDueAt) {
		return response, nil
	}
	asset, err := s.media.FindAsset(ctx, moment.MediaAssetID, false)
	if err != nil {
		return nil, appErrors.InternalError
	}
	signedURL, err := s.storage.PresignDownload(ctx, asset, signingTime, mediaReadLifetime)
	if err != nil {
		return nil, mediaUnavailableError()
	}
	expiresAt := signingTime.Add(mediaReadLifetime).UTC()
	response.ImageURL = signedURL
	response.ThumbnailURL = signedURL
	response.ShareImageURL = signedURL
	response.ImageExpiresAt = &expiresAt
	return response, nil
}

type momentCursorPayload struct {
	CapturedAt string `json:"capturedAt"`
	ID         string `json:"id"`
}

func (s *MomentService) encodeCursor(moment *momentEntities.Moment) (string, error) {
	secret := strings.TrimSpace(s.cursorSecret())
	if secret == "" {
		return "", appErrors.InternalError
	}
	payload, err := json.Marshal(momentCursorPayload{
		CapturedAt: moment.CapturedAt.UTC().Format(time.RFC3339Nano),
		ID:         moment.ID,
	})
	if err != nil {
		return "", appErrors.InternalError
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(encoded))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return encoded + "." + signature, nil
}

func (s *MomentService) decodeCursor(raw string) (*momentEntities.Cursor, error) {
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ".")
	secret := strings.TrimSpace(s.cursorSecret())
	if len(parts) != 2 || secret == "" {
		return nil, invalidCursorError()
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, invalidCursorError()
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(parts[0]))
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return nil, invalidCursorError()
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, invalidCursorError()
	}
	var payload momentCursorPayload
	if json.Unmarshal(payloadBytes, &payload) != nil {
		return nil, invalidCursorError()
	}
	capturedAt, err := time.Parse(time.RFC3339Nano, payload.CapturedAt)
	if err != nil {
		return nil, invalidCursorError()
	}
	id, err := uuid.Parse(payload.ID)
	if err != nil {
		return nil, invalidCursorError()
	}
	return &momentEntities.Cursor{CapturedAt: capturedAt.UTC(), ID: id.String()}, nil
}

func invalidCursorError() error {
	return mediaMomentError(http.StatusBadRequest, "INVALID_REQUEST", "cursor inválido.")
}

func (s *MomentService) List(
	ctx context.Context,
	scope string,
	rawCursor string,
) (*messages.MomentPageResponseDTO, error) {
	if scope != "feed" && scope != "mine" && scope != "group" {
		return nil, mediaMomentError(
			http.StatusBadRequest,
			"INVALID_REQUEST",
			"scope deve ser feed, mine ou group.",
		)
	}
	actor, err := requireOnboardedActor(ctx, s.users, false)
	if err != nil {
		return nil, err
	}
	cursor, err := s.decodeCursor(rawCursor)
	if err != nil {
		return nil, err
	}
	now := utcNow(s.now)
	page, err := s.moments.ListMoments(ctx, scope, actor.ID, actor.GroupID, cursor, now)
	if err != nil {
		return nil, appErrors.InternalError
	}

	response := &messages.MomentPageResponseDTO{
		Items: make([]messages.MomentResponseDTO, len(page.Items)),
	}
	for index := range page.Items {
		item, mapErr := s.responseFor(ctx, &page.Items[index], now)
		if mapErr != nil {
			return nil, mapErr
		}
		response.Items[index] = *item
	}
	if page.HasNext && len(page.Items) > 0 {
		nextCursor, cursorErr := s.encodeCursor(&page.Items[len(page.Items)-1])
		if cursorErr != nil {
			return nil, cursorErr
		}
		response.NextCursor = &nextCursor
	}
	return response, nil
}

type createMomentIntent struct {
	MediaAssetID    string  `json:"mediaAssetId"`
	PublishConsent  bool    `json:"publishConsent"`
	ParticipationID *string `json:"participationId,omitempty"`
	ChallengeMode   bool    `json:"challengeMode,omitempty"`
}

func (s *MomentService) Create(
	ctx context.Context,
	rawKey string,
	request *messages.CreateMomentRequestDTO,
) (*messages.MomentResponseDTO, int, error) {
	if request == nil {
		return nil, 0, mediaMomentError(http.StatusBadRequest, "INVALID_REQUEST", "Corpo obrigatório.")
	}
	assetID, err := uuid.Parse(request.MediaAssetID)
	if err != nil {
		return nil, 0, notFoundError()
	}
	participationID, err := normalizeOptionalUUID(request.ParticipationID)
	if err != nil {
		return nil, 0, notFoundError()
	}
	key, err := parseIdempotencyKey(rawKey)
	if err != nil {
		return nil, 0, err
	}
	actor, err := requireOnboardedActor(ctx, s.users, false)
	if err != nil {
		return nil, 0, err
	}

	operation := "moment.create"
	fingerprint := intentHash(operation, createMomentIntent{
		MediaAssetID:    assetID.String(),
		PublishConsent:  request.PublishConsent,
		ParticipationID: participationID,
		ChallengeMode:   request.ChallengeMode,
	})
	now := utcNow(s.now)

	// Every read runs outside the transaction and without row locks, mirroring
	// GameService.ValidateQR. The unique indexes on moments and point_entries
	// already reject duplicates, so the old FOR UPDATE on the challenge
	// activity only queued every publishing participant behind one another and
	// collided with the FOR KEY SHARE their own inserts take on that same row.
	asset, findErr := s.media.FindAsset(ctx, assetID.String(), false)
	if errors.Is(findErr, appErrors.ErrNotFound) || findErr == nil && asset.OwnerUserID != actor.ID {
		return nil, 0, notFoundError()
	}
	if findErr != nil {
		return nil, 0, appErrors.InternalError
	}
	prior, priorErr := findIdempotencyOperation(ctx, s.media, actor.ID, key, operation, fingerprint)
	if priorErr != nil {
		return nil, 0, priorErr
	}
	if prior != nil {
		if prior.ResultRef == nil {
			return nil, 0, appErrors.InternalError
		}
		replayed, momentErr := s.moments.FindMoment(ctx, *prior.ResultRef, actor.ID, false)
		if momentErr != nil {
			return nil, 0, appErrors.InternalError
		}
		response, err := s.responseFor(ctx, replayed, prior.CreatedAt.UTC())
		if err != nil {
			return nil, 0, err
		}
		return response, prior.HTTPStatus, nil
	}
	if asset.State != mediaEntities.AssetAvailable || !now.Before(asset.RetentionDueAt) {
		return nil, 0, mediaMomentError(
			http.StatusConflict,
			"UPLOAD_STATE_CONFLICT",
			"O asset não está disponível.",
		)
	}

	origin := momentEntities.OriginFree
	rewardStatus := momentEntities.RewardNotApplicable
	var activityID *string
	points := 0
	if participationID != nil {
		if closed, csErr := s.eventSettings.Get(ctx); csErr == nil && closed.ScoringClosed {
			return nil, 0, mediaMomentError(http.StatusForbidden, "SCORING_CLOSED", "A pontuação está fechada.")
		}
		participation, activityPoints, eligibilityErr := s.eligibleParticipation(
			ctx,
			*participationID,
			actor.ID,
			now,
		)
		if eligibilityErr != nil {
			return nil, 0, eligibilityErr
		}
		origin = momentEntities.OriginChallenge
		rewardStatus = momentEntities.RewardDenied
		activityID = &participation.ActivityID
		if request.PublishConsent {
			points = activityPoints
		}
	} else if request.ChallengeMode {
		if closed, csErr := s.eventSettings.Get(ctx); csErr == nil && closed.ScoringClosed {
			return nil, 0, mediaMomentError(http.StatusForbidden, "SCORING_CLOSED", "A pontuação está fechada.")
		}
		repo, ok := s.moments.(activeMomentChallengeRepository)
		if !ok {
			return nil, 0, appErrors.InternalError
		}
		challengeID, challengePoints, challengeErr := repo.FindActiveMomentChallengeForUpdate(ctx, now)
		if errors.Is(challengeErr, appErrors.ErrNotFound) || errors.Is(challengeErr, appErrors.ErrConflict) {
			return nil, 0, mediaMomentError(http.StatusConflict, "MOMENT_UNAVAILABLE", "Não há desafio do momento ativo agora.")
		}
		if challengeErr != nil {
			return nil, 0, appErrors.InternalError
		}
		already, duplicateErr := repo.HasMomentForActivity(ctx, actor.ID, challengeID)
		if duplicateErr != nil {
			return nil, 0, appErrors.InternalError
		}
		if already {
			return nil, 0, challengeAlreadyCompletedError()
		}
		origin = momentEntities.OriginChallenge
		rewardStatus = momentEntities.RewardDenied
		activityID = &challengeID
		points = challengePoints
	} else {
		// A spontaneous share earns a flat reward when it goes to the feed and
		// scoring is still open; publishing itself is never blocked.
		rewardStatus = momentEntities.RewardDenied
		if request.PublishConsent {
			points = freeMomentPoints
			if closed, csErr := s.eventSettings.Get(ctx); csErr == nil && closed.ScoringClosed {
				points = 0
			}
		}
	}
	// Only participants (DEFAULT) compete: staff may publish moments to
	// share the experience, but never receive points.
	if actor.Role != userEntities.RoleDefault {
		points = 0
	}

	publicationStatus := momentEntities.PublicationPrivate
	if request.PublishConsent {
		publicationStatus = momentEntities.PublicationPublic
	}
	moment := &momentEntities.Moment{
		ID:                uuid.NewString(),
		UserID:            actor.ID,
		ParticipationID:   participationID,
		ActivityID:        activityID,
		MediaAssetID:      asset.ID,
		Origin:            origin,
		PublicationStatus: publicationStatus,
		ModerationStatus:  momentEntities.ModerationPending,
		RewardStatus:      rewardStatus,
		CapturedAt:        now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	duplicate := false
	err = s.WithTransaction(ctx, func(tx context.Context) error {
		// Write phase only: a concurrent duplicate surfaces here as a unique
		// violation instead of being fenced by a row lock.
		if createErr := s.moments.CreateMoment(tx, moment); createErr != nil {
			if errors.Is(createErr, appErrors.ErrConflict) {
				duplicate = true
				if request.ChallengeMode {
					return challengeAlreadyCompletedError()
				}
				return momentAlreadyCreatedError()
			}
			return appErrors.InternalError
		}
		activityRef := ""
		if activityID != nil {
			activityRef = *activityID
		}
		if awardErr := s.moments.AwardMoment(tx, moment.ID, actor.ID, activityRef, points, now); awardErr != nil {
			if errors.Is(awardErr, appErrors.ErrConflict) {
				duplicate = true
				if request.ChallengeMode {
					return challengeAlreadyCompletedError()
				}
				return momentAlreadyCreatedError()
			}
			return appErrors.InternalError
		}
		completedAt := now
		if createErr := createIdempotencyOperation(tx, s.media, &mediaEntities.Operation{
			ID:               uuid.NewString(),
			ActorUserID:      actor.ID,
			IdempotencyKey:   key,
			Operation:        operation,
			ResourceRef:      &asset.ID,
			IntentHash:       fingerprint,
			State:            "completed",
			ResultRef:        &moment.ID,
			ResponseSnapshot: []byte(`{}`),
			HTTPStatus:       http.StatusCreated,
			CreatedAt:        now,
			CompletedAt:      &completedAt,
		}); createErr != nil {
			return appErrors.InternalError
		}
		return nil
	})
	if errors.Is(err, errIdempotencyRace) {
		return s.Create(ctx, rawKey, request)
	}
	if err != nil {
		if duplicate {
			// A retry carrying the same Idempotency-Key may have raced the request
			// that created the Moment: replay that result instead of a conflict.
			if prior, priorErr := findIdempotencyOperation(ctx, s.media, actor.ID, key, operation, fingerprint); priorErr == nil && prior != nil {
				return s.Create(ctx, rawKey, request)
			}
		}
		return nil, 0, err
	}
	created, findErr := s.moments.FindMoment(ctx, moment.ID, actor.ID, false)
	if findErr != nil {
		return nil, 0, appErrors.InternalError
	}
	response, err := s.responseFor(ctx, created, now)
	if err != nil {
		return nil, 0, err
	}
	return response, http.StatusCreated, nil
}

func (s *MomentService) eligibleParticipation(
	ctx context.Context,
	participationID string,
	actorID uint64,
	now time.Time,
) (*gameEntities.Participation, int, error) {
	participation, err := s.moments.FindParticipationForUpdate(ctx, participationID)
	if errors.Is(err, appErrors.ErrNotFound) || err == nil && participation.UserID != actorID {
		return nil, 0, notFoundError()
	}
	if err != nil {
		return nil, 0, appErrors.InternalError
	}
	status, allowsMoment, startsAt, endsAt, momentPoints, _, _, err := s.moments.FindActivityForUpdate(
		ctx,
		participation.ActivityID,
	)
	if err != nil {
		return nil, 0, appErrors.InternalError
	}
	eligible := participation.Status != "cancelled" &&
		participation.CanShareMoment &&
		allowsMoment &&
		status == "active" &&
		(startsAt == nil || !now.Before(startsAt.UTC())) &&
		(endsAt == nil || !now.After(endsAt.UTC()))
	if !eligible {
		return nil, 0, mediaMomentError(
			http.StatusConflict,
			"MOMENT_NOT_ELIGIBLE",
			"Esta participação não permite publicar um Moment agora.",
		)
	}
	return participation, momentPoints, nil
}

func normalizeOptionalUUID(raw *string) (*string, error) {
	if raw == nil {
		return nil, nil
	}
	id, err := uuid.Parse(*raw)
	if err != nil {
		return nil, err
	}
	normalized := id.String()
	return &normalized, nil
}

func notFoundError() error {
	return mediaMomentError(http.StatusNotFound, "NOT_FOUND", "Recurso não encontrado.")
}

func momentAlreadyCreatedError() error {
	return mediaMomentError(
		http.StatusConflict,
		"MOMENT_ALREADY_CREATED",
		"Este asset ou Participation já possui um Moment.",
	)
}

func challengeAlreadyCompletedError() error {
	return mediaMomentError(
		http.StatusConflict,
		"MOMENT_ALREADY_COMPLETED",
		"Você já concluiu este desafio do momento.",
	)
}
