package services

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"time"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	appInterfaces "github.com/dnjtechteam/dnj-game-api/internal/app/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	activityEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/activity/entities"
	activityInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/activity/interfaces"
	auditEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/operationaudit/entities"
	auditInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/operationaudit/interfaces"
	userEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
	userInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/user/interfaces"
	"github.com/google/uuid"
)

type ActivityService struct {
	*BaseService
	activities activityInterfaces.ActivityRepositoryInterface
	audits     auditInterfaces.OperationAuditRepositoryInterface
	users      userInterfaces.UserRepositoryInterface
	now        func() time.Time
}

func NewActivityService(base *BaseService, activities activityInterfaces.ActivityRepositoryInterface, audits auditInterfaces.OperationAuditRepositoryInterface, users userInterfaces.UserRepositoryInterface) appInterfaces.ActivityServiceInterface {
	return &ActivityService{BaseService: base, activities: activities, audits: audits, users: users, now: time.Now}
}

type activityTransitionMetadata struct {
	FromStatus string `json:"fromStatus"`
	ToStatus   string `json:"toStatus"`
}

type scheduleOperationMetadata struct {
	FromStatus      string     `json:"fromStatus"`
	ToStatus        string     `json:"toStatus"`
	ActualStartedAt *time.Time `json:"actualStartedAt,omitempty"`
	FlexMinutes     int        `json:"flexMinutes"`
}

func activityOperationError(status int, code, message string) error {
	return appErrors.NewAPIServiceError(status, code, message, nil)
}

func (s *ActivityService) transition(ctx context.Context, rawActivityID, rawKey, action string, allowed []activityEntities.Status, target activityEntities.Status, allowedKinds []activityEntities.Kind) (*messages.ActivityStateResponseDTO, error) {
	activityUUID, err := uuid.Parse(rawActivityID)
	if err != nil {
		return nil, activityOperationError(http.StatusNotFound, "NOT_FOUND", "Atividade não encontrada.")
	}
	keyUUID, err := uuid.Parse(rawKey)
	if err != nil {
		return nil, activityOperationError(http.StatusBadRequest, "INVALID_REQUEST", "Idempotency-Key deve ser um UUID válido.")
	}
	actorID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}

	activityID := activityUUID.String()
	idempotencyKey := keyUUID.String()
	var response *messages.ActivityStateResponseDTO
	err = s.WithTransaction(ctx, func(txCtx context.Context) error {
		actor, findErr := s.users.FindByIDForUpdate(txCtx, actorID)
		if errors.Is(findErr, appErrors.ErrNotFound) {
			return activityOperationError(http.StatusUnauthorized, "UNAUTHENTICATED", "Autenticação necessária.")
		}
		if findErr != nil {
			return appErrors.InternalError
		}
		if actor.Role == userEntities.RoleDefault || (actor.Role != userEntities.RoleAdmin && actor.Role != userEntities.RoleEventManager) {
			return activityOperationError(http.StatusForbidden, "FORBIDDEN", "Operação não permitida para este papel.")
		}
		activity, findErr := s.activities.FindAuthorizedForUpdate(txCtx, activityID, actorID, actor.Role == userEntities.RoleAdmin)
		if errors.Is(findErr, appErrors.ErrNotFound) {
			return activityOperationError(http.StatusNotFound, "NOT_FOUND", "Atividade não encontrada.")
		}
		if findErr != nil {
			return appErrors.InternalError
		}

		prior, auditErr := s.audits.FindByActorAndIdempotencyKey(txCtx, actorID, idempotencyKey)
		if auditErr == nil {
			if prior.Action != action || prior.EntityID == nil || *prior.EntityID != activityID {
				return activityOperationError(http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "Idempotency-Key já foi usado em outra operação.")
			}
			var metadata activityTransitionMetadata
			if json.Unmarshal(prior.Metadata, &metadata) != nil || metadata.ToStatus == "" {
				return appErrors.InternalError
			}
			response = &messages.ActivityStateResponseDTO{ID: activityID, Status: metadata.ToStatus}
			return nil
		}
		if !errors.Is(auditErr, appErrors.ErrNotFound) {
			return appErrors.InternalError
		}

		if !slices.Contains(allowed, activity.Status) {
			return activityOperationError(http.StatusConflict, "ACTIVITY_STATE_CONFLICT", "A atividade não pode executar esta transição no estado atual.")
		}
		if allowedKinds != nil && !slices.Contains(allowedKinds, activity.Kind) {
			return activityOperationError(http.StatusConflict, "ACTIVITY_STATE_CONFLICT", "Esta transição não é permitida para o kind da atividade.")
		}
		now := s.now().UTC()
		if updateErr := s.activities.TransitionStatus(txCtx, activityID, activity.Status, target, now); updateErr != nil {
			if errors.Is(updateErr, appErrors.ErrConflict) {
				return activityOperationError(http.StatusConflict, "ACTIVITY_STATE_CONFLICT", "A atividade mudou de estado durante a operação.")
			}
			return appErrors.InternalError
		}
		metadata, marshalErr := json.Marshal(activityTransitionMetadata{FromStatus: string(activity.Status), ToStatus: string(target)})
		if marshalErr != nil {
			return appErrors.InternalError
		}
		entityID := activityID
		if _, auditErr = s.audits.Create(txCtx, &auditEntities.OperationAudit{ID: uuid.NewString(), ActorUserID: &actorID, Action: action, EntityType: "activity", EntityID: &entityID, Metadata: metadata, IdempotencyKey: idempotencyKey, CreatedAt: now}); auditErr != nil {
			if errors.Is(auditErr, appErrors.ErrConflict) {
				return activityOperationError(http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "Idempotency-Key já foi usado em outra operação.")
			}
			return appErrors.InternalError
		}
		response = &messages.ActivityStateResponseDTO{ID: activityID, Status: string(target)}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (s *ActivityService) Start(ctx context.Context, activityID, idempotencyKey string) (*messages.ActivityStateResponseDTO, error) {
	return s.transition(ctx, activityID, idempotencyKey, "activity.start", []activityEntities.Status{activityEntities.StatusDraft, activityEntities.StatusPaused}, activityEntities.StatusActive, nil)
}

func (s *ActivityService) Pause(ctx context.Context, activityID, idempotencyKey string) (*messages.ActivityStateResponseDTO, error) {
	return s.transition(ctx, activityID, idempotencyKey, "activity.pause", []activityEntities.Status{activityEntities.StatusActive}, activityEntities.StatusPaused, nil)
}

func (s *ActivityService) Conclude(ctx context.Context, activityID, idempotencyKey string) (*messages.ActivityStateResponseDTO, error) {
	return s.transition(ctx, activityID, idempotencyKey, "activity.conclude", []activityEntities.Status{activityEntities.StatusActive, activityEntities.StatusPaused}, activityEntities.StatusCompleted, []activityEntities.Kind{activityEntities.KindChallenge, activityEntities.KindCompetitive, activityEntities.KindLive})
}

func (s *ActivityService) scheduleOperation(ctx context.Context, rawActivityID, rawKey, action string) (*messages.ActivityStateResponseDTO, error) {
	activityUUID, err := uuid.Parse(rawActivityID)
	if err != nil {
		return nil, activityOperationError(http.StatusNotFound, "NOT_FOUND", "Atividade não encontrada.")
	}
	keyUUID, err := uuid.Parse(rawKey)
	if err != nil {
		return nil, activityOperationError(http.StatusBadRequest, "INVALID_REQUEST", "Idempotency-Key deve ser um UUID válido.")
	}
	actorID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	var response *messages.ActivityStateResponseDTO
	err = s.WithTransaction(ctx, func(txCtx context.Context) error {
		actor, findErr := s.users.FindByIDForUpdate(txCtx, actorID)
		if errors.Is(findErr, appErrors.ErrNotFound) {
			return activityOperationError(http.StatusUnauthorized, "UNAUTHENTICATED", "Autenticação necessária.")
		}
		if findErr != nil {
			return appErrors.InternalError
		}
		if actor.Role == userEntities.RoleDefault || (actor.Role != userEntities.RoleAdmin && actor.Role != userEntities.RoleEventManager) {
			return activityOperationError(http.StatusForbidden, "FORBIDDEN", "Operação não permitida para este papel.")
		}
		if actor.Role == userEntities.RoleEventManager && (actor.ManagerScope == nil || *actor.ManagerScope != "space") {
			return activityOperationError(http.StatusForbidden, "FORBIDDEN", "Esta operação exige o escopo de Cronograma.")
		}
		activity, findErr := s.activities.FindAuthorizedForUpdate(txCtx, activityUUID.String(), actorID, actor.Role == userEntities.RoleAdmin)
		if errors.Is(findErr, appErrors.ErrNotFound) {
			return activityOperationError(http.StatusNotFound, "NOT_FOUND", "Atividade não encontrada.")
		}
		if findErr != nil {
			return appErrors.InternalError
		}
		if activity.Kind != activityEntities.KindSchedule {
			return activityOperationError(http.StatusConflict, "ACTIVITY_STATE_CONFLICT", "Esta operação é permitida somente para atividades de agenda.")
		}
		idempotencyKey := keyUUID.String()
		prior, auditErr := s.audits.FindByActorAndIdempotencyKey(txCtx, actorID, idempotencyKey)
		if auditErr == nil {
			if prior.Action != action || prior.EntityID == nil || *prior.EntityID != activity.ID {
				return activityOperationError(http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "Idempotency-Key já foi usado em outra operação.")
			}
			var metadata scheduleOperationMetadata
			if json.Unmarshal(prior.Metadata, &metadata) != nil {
				return appErrors.InternalError
			}
			response = &messages.ActivityStateResponseDTO{ID: activity.ID, Status: metadata.ToStatus}
			return nil
		}
		if !errors.Is(auditErr, appErrors.ErrNotFound) {
			return appErrors.InternalError
		}
		fromStatus := activity.Status
		switch action {
		case "schedule.start":
			if activity.ActualStartedAt != nil {
				return activityOperationError(http.StatusConflict, "ACTIVITY_STATE_CONFLICT", "O início real desta atividade já foi registrado.")
			}
			if activity.Status != activityEntities.StatusActive && activity.Status != activityEntities.StatusPaused {
				return activityOperationError(http.StatusConflict, "ACTIVITY_STATE_CONFLICT", "A atividade não está disponível para início.")
			}
			startedAt := s.now().UTC()
			activity.ActualStartedAt = &startedAt
		case "schedule.flex":
			if activity.ActualStartedAt == nil || activity.Status == activityEntities.StatusCompleted {
				return activityOperationError(http.StatusConflict, "ACTIVITY_STATE_CONFLICT", "Inicie uma atividade de agenda antes de aplicar o Flex time.")
			}
			activity.FlexMinutes = 15
		case "schedule.advance":
			if activity.Status != activityEntities.StatusActive && activity.Status != activityEntities.StatusPaused {
				return activityOperationError(http.StatusConflict, "ACTIVITY_STATE_CONFLICT", "A atividade não pode avançar neste estado.")
			}
			activity.Status = activityEntities.StatusCompleted
		default:
			return appErrors.InternalError
		}
		now := s.now().UTC()
		activity.UpdatedAt = now
		if _, updateErr := s.activities.Update(txCtx, activity); updateErr != nil {
			return appErrors.InternalError
		}
		metadata, marshalErr := json.Marshal(scheduleOperationMetadata{FromStatus: string(fromStatus), ToStatus: string(activity.Status), ActualStartedAt: activity.ActualStartedAt, FlexMinutes: activity.FlexMinutes})
		if marshalErr != nil {
			return appErrors.InternalError
		}
		entityID := activity.ID
		if _, auditErr = s.audits.Create(txCtx, &auditEntities.OperationAudit{ID: uuid.NewString(), ActorUserID: &actorID, Action: action, EntityType: "activity", EntityID: &entityID, Metadata: metadata, IdempotencyKey: idempotencyKey, CreatedAt: now}); auditErr != nil {
			if errors.Is(auditErr, appErrors.ErrConflict) {
				return activityOperationError(http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "Idempotency-Key já foi usado em outra operação.")
			}
			return appErrors.InternalError
		}
		response = &messages.ActivityStateResponseDTO{ID: activity.ID, Status: string(activity.Status)}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (s *ActivityService) StartScheduled(ctx context.Context, activityID, idempotencyKey string) (*messages.ActivityStateResponseDTO, error) {
	return s.scheduleOperation(ctx, activityID, idempotencyKey, "schedule.start")
}

func (s *ActivityService) FlexScheduled(ctx context.Context, activityID, idempotencyKey string) (*messages.ActivityStateResponseDTO, error) {
	return s.scheduleOperation(ctx, activityID, idempotencyKey, "schedule.flex")
}

func (s *ActivityService) AdvanceScheduled(ctx context.Context, activityID, idempotencyKey string) (*messages.ActivityStateResponseDTO, error) {
	return s.scheduleOperation(ctx, activityID, idempotencyKey, "schedule.advance")
}
