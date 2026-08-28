package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	appInterfaces "github.com/dnjtechteam/dnj-game-api/internal/app/interfaces"
	appMappers "github.com/dnjtechteam/dnj-game-api/internal/app/mappers"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	activityEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/activity/entities"
	activityInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/activity/interfaces"
	adminEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/adminoperation/entities"
	adminInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/adminoperation/interfaces"
	auditEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/operationaudit/entities"
	auditInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/operationaudit/interfaces"
	spaceEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/space/entities"
	spaceInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/space/interfaces"
	userEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
	userInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/user/interfaces"
	"github.com/google/uuid"
)

var adminSlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type AdminInstallationService struct {
	*BaseService
	spaces     spaceInterfaces.SpaceRepositoryInterface
	activities activityInterfaces.ActivityRepositoryInterface
	audits     auditInterfaces.OperationAuditRepositoryInterface
	operations adminInterfaces.AdminOperationRepositoryInterface
	users      userInterfaces.UserRepositoryInterface
	now        func() time.Time
}

func NewAdminInstallationService(base *BaseService, spaces spaceInterfaces.SpaceRepositoryInterface, activities activityInterfaces.ActivityRepositoryInterface, audits auditInterfaces.OperationAuditRepositoryInterface, operations adminInterfaces.AdminOperationRepositoryInterface, users userInterfaces.UserRepositoryInterface) appInterfaces.AdminInstallationServiceInterface {
	return &AdminInstallationService{BaseService: base, spaces: spaces, activities: activities, audits: audits, operations: operations, users: users, now: time.Now}
}

func adminAPIError(status int, code, message string) error {
	return appErrors.NewAPIServiceError(status, code, message, nil)
}

func (s *AdminInstallationService) authorizeAdmin(ctx context.Context, lock bool) (uint64, error) {
	actorID, err := authenticatedUserID(ctx)
	if err != nil {
		return 0, err
	}
	var actor *userEntities.User
	if lock {
		actor, err = s.users.FindByIDForUpdate(ctx, actorID)
	} else {
		actor, err = s.users.FindByID(ctx, actorID)
	}
	if errors.Is(err, appErrors.ErrNotFound) {
		return 0, adminAPIError(http.StatusUnauthorized, "UNAUTHENTICATED", "Autenticação necessária.")
	}
	if err != nil {
		return 0, appErrors.InternalError
	}
	if actor.Role != userEntities.RoleAdmin {
		return 0, adminAPIError(http.StatusForbidden, "FORBIDDEN", "Operação permitida somente para ADMIN.")
	}
	return actorID, nil
}

type adminWriteOutcome[T any] struct {
	Response      *T
	EntityType    string
	EntityRef     string
	EntityID      *string
	AuditMetadata any
	HTTPStatus    int
}

func adminIntentHash(operation string, intent any) string {
	encoded, _ := json.Marshal(intent)
	digest := sha256.Sum256(append([]byte(operation+"\x00"), encoded...))
	return hex.EncodeToString(digest[:])
}

func runAdminWrite[T any](s *AdminInstallationService, ctx context.Context, rawKey, operation string, intent any, apply func(context.Context, uint64) (*adminWriteOutcome[T], error)) (*T, error) {
	var response *T
	err := s.WithTransaction(ctx, func(txCtx context.Context) error {
		actorID, authErr := s.authorizeAdmin(txCtx, true)
		if authErr != nil {
			return authErr
		}
		key, keyErr := uuid.Parse(rawKey)
		if keyErr != nil {
			return adminAPIError(http.StatusBadRequest, "INVALID_REQUEST", "Idempotency-Key deve ser um UUID válido.")
		}
		idempotencyKey := key.String()
		requestHash := adminIntentHash(operation, intent)

		prior, findErr := s.operations.FindByActorAndIdempotencyKey(txCtx, actorID, idempotencyKey)
		if findErr == nil {
			if prior.Operation != operation || prior.RequestHash != requestHash {
				return adminAPIError(http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "Idempotency-Key já foi usado em outra operação.")
			}
			var original T
			if err := json.Unmarshal(prior.Response, &original); err != nil {
				return appErrors.InternalError
			}
			response = &original
			return nil
		}
		if !errors.Is(findErr, appErrors.ErrNotFound) {
			return appErrors.InternalError
		}
		if _, auditErr := s.audits.FindByActorAndIdempotencyKey(txCtx, actorID, idempotencyKey); auditErr == nil {
			return adminAPIError(http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "Idempotency-Key já foi usado em outra operação.")
		} else if !errors.Is(auditErr, appErrors.ErrNotFound) {
			return appErrors.InternalError
		}

		outcome, applyErr := apply(txCtx, actorID)
		if applyErr != nil {
			return applyErr
		}
		if outcome == nil || outcome.Response == nil || outcome.EntityRef == "" {
			return appErrors.InternalError
		}
		responseJSON, marshalErr := json.Marshal(outcome.Response)
		if marshalErr != nil {
			return appErrors.InternalError
		}
		metadataJSON, marshalErr := json.Marshal(outcome.AuditMetadata)
		if marshalErr != nil {
			return appErrors.InternalError
		}
		now := s.now().UTC()
		if _, createErr := s.operations.Create(txCtx, &adminEntities.AdminOperation{ID: uuid.NewString(), ActorUserID: actorID, IdempotencyKey: idempotencyKey, Operation: operation, EntityType: outcome.EntityType, EntityRef: outcome.EntityRef, RequestHash: requestHash, HTTPStatus: outcome.HTTPStatus, Response: responseJSON, CreatedAt: now}); createErr != nil {
			if errors.Is(createErr, appErrors.ErrConflict) {
				return adminAPIError(http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "Idempotency-Key já foi usado em outra operação.")
			}
			return appErrors.InternalError
		}
		entityReference := outcome.EntityRef
		if _, createErr := s.audits.Create(txCtx, &auditEntities.OperationAudit{ID: uuid.NewString(), ActorUserID: &actorID, Action: operation, EntityType: outcome.EntityType, EntityID: outcome.EntityID, EntityReference: &entityReference, Metadata: metadataJSON, IdempotencyKey: idempotencyKey, CreatedAt: now}); createErr != nil {
			if errors.Is(createErr, appErrors.ErrConflict) {
				return adminAPIError(http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "Idempotency-Key já foi usado em outra operação.")
			}
			return appErrors.InternalError
		}
		response = outcome.Response
		return nil
	})
	if err != nil {
		return nil, err
	}
	return response, nil
}

func validateAdminSlug(slug *string) (string, error) {
	if slug == nil {
		return "", adminAPIError(http.StatusBadRequest, "INVALID_REQUEST", "slug é obrigatório.")
	}
	value := strings.TrimSpace(*slug)
	if len(value) == 0 || len(value) > 120 || !adminSlugPattern.MatchString(value) {
		return "", adminAPIError(http.StatusBadRequest, "INVALID_REQUEST", "slug deve ter até 120 caracteres e usar letras minúsculas, números e hífens.")
	}
	return value, nil
}

func validateAdminName(name *string) (string, error) {
	if name == nil {
		return "", adminAPIError(http.StatusBadRequest, "INVALID_REQUEST", "name é obrigatório.")
	}
	value := strings.TrimSpace(*name)
	if len(value) == 0 || len(value) > 200 {
		return "", adminAPIError(http.StatusBadRequest, "INVALID_REQUEST", "name deve ter entre 1 e 200 caracteres.")
	}
	return value, nil
}

func validateOptionalText(value *string, max int, field string) (*string, error) {
	if value == nil {
		return nil, nil
	}
	trimmed := strings.TrimSpace(*value)
	if len(trimmed) > max {
		return nil, adminAPIError(http.StatusBadRequest, "INVALID_REQUEST", field+" excede o tamanho permitido.")
	}
	return &trimmed, nil
}

func adminUTCTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	normalized := value.UTC()
	return &normalized
}

func mapAdminActivity(activity *activityEntities.Activity) *messages.AdminActivityResponseDTO {
	return &messages.AdminActivityResponseDTO{ID: activity.ID, SpaceID: activity.SpaceID, Slug: activity.Slug, Name: activity.Name, Description: activity.Description, Kind: string(activity.Kind), Status: string(activity.Status), StartsAt: adminUTCTime(activity.StartsAt), EndsAt: adminUTCTime(activity.EndsAt), CheckInPoints: activity.CheckInPoints, MomentPoints: activity.MomentPoints, CooldownSeconds: activity.CooldownSeconds, AllowsMoment: activity.AllowsMoment}
}

func mapAdminStaff(user *userEntities.User) messages.AdminStaffResponseDTO {
	scope := ""
	if user.ManagerScope != nil {
		scope = *user.ManagerScope
	}
	return messages.AdminStaffResponseDTO{ID: messages.Uint64StringFromUint64(user.ID), Name: user.Name, Email: user.Email, Role: string(user.Role), Scope: scope, OnboardingComplete: user.OnboardingComplete}
}

func (s *AdminInstallationService) ListSpaces(ctx context.Context, filter *messages.ListAdminSpacesFilterDTO) (*messages.PaginatedResponse[messages.SpaceResponseDTO], error) {
	if _, err := s.authorizeAdmin(ctx, false); err != nil {
		return nil, err
	}
	result, err := s.spaces.List(ctx, filter.GetPage())
	if err != nil {
		return nil, appErrors.InternalError
	}
	items := make([]messages.SpaceResponseDTO, len(result.Data))
	for index := range result.Data {
		items[index] = *appMappers.MapSpaceToResponseDTO(&result.Data[index])
	}
	return &messages.PaginatedResponse[messages.SpaceResponseDTO]{Data: items, Pagination: result.Pagination}, nil
}

func (s *AdminInstallationService) CreateSpace(ctx context.Context, key string, request *messages.CreateAdminSpaceRequestDTO) (*messages.SpaceResponseDTO, error) {
	return runAdminWrite(s, ctx, key, "admin.space.create", request, func(txCtx context.Context, _ uint64) (*adminWriteOutcome[messages.SpaceResponseDTO], error) {
		if request == nil {
			return nil, adminAPIError(http.StatusBadRequest, "INVALID_REQUEST", "Corpo obrigatório.")
		}
		slug, err := validateAdminSlug(request.Slug)
		if err != nil {
			return nil, err
		}
		name, err := validateAdminName(request.Name)
		if err != nil {
			return nil, err
		}
		mapReference, err := validateOptionalText(request.MapReference.Value, 1000, "mapReference")
		if err != nil {
			return nil, err
		}
		now := s.now().UTC()
		created, err := s.spaces.Create(txCtx, &spaceEntities.Space{ID: uuid.NewString(), Slug: slug, Name: name, MapReference: mapReference, CreatedAt: now, UpdatedAt: now})
		if errors.Is(err, appErrors.ErrConflict) {
			return nil, adminAPIError(http.StatusConflict, "SLUG_ALREADY_EXISTS", "Já existe um Space com este slug.")
		}
		if err != nil {
			return nil, appErrors.InternalError
		}
		response := appMappers.MapSpaceToResponseDTO(created)
		entityID := created.ID
		return &adminWriteOutcome[messages.SpaceResponseDTO]{Response: response, EntityType: "space", EntityRef: created.ID, EntityID: &entityID, AuditMetadata: map[string]any{"created": true}, HTTPStatus: http.StatusCreated}, nil
	})
}

func (s *AdminInstallationService) UpdateSpace(ctx context.Context, rawSpaceID, key string, request *messages.UpdateAdminSpaceRequestDTO) (*messages.SpaceResponseDTO, error) {
	intent := struct {
		ID      string
		Request *messages.UpdateAdminSpaceRequestDTO
	}{rawSpaceID, request}
	return runAdminWrite(s, ctx, key, "admin.space.update", intent, func(txCtx context.Context, _ uint64) (*adminWriteOutcome[messages.SpaceResponseDTO], error) {
		spaceID, err := uuid.Parse(rawSpaceID)
		if err != nil {
			return nil, adminAPIError(http.StatusNotFound, "NOT_FOUND", "Space não encontrado.")
		}
		if request == nil || (!request.Slug.Set && !request.Name.Set && !request.MapReference.Set) {
			return nil, adminAPIError(http.StatusBadRequest, "INVALID_REQUEST", "Informe ao menos um campo para alteração.")
		}
		current, err := s.spaces.FindByIDForUpdate(txCtx, spaceID.String())
		if errors.Is(err, appErrors.ErrNotFound) {
			return nil, adminAPIError(http.StatusNotFound, "NOT_FOUND", "Space não encontrado.")
		}
		if err != nil {
			return nil, appErrors.InternalError
		}
		before := *current
		changedFields := make([]string, 0, 3)
		if request.Slug.Set {
			value, validationErr := validateAdminSlug(request.Slug.Value)
			if validationErr != nil {
				return nil, validationErr
			}
			current.Slug = value
			changedFields = append(changedFields, "slug")
		}
		if request.Name.Set {
			value, validationErr := validateAdminName(request.Name.Value)
			if validationErr != nil {
				return nil, validationErr
			}
			current.Name = value
			changedFields = append(changedFields, "name")
		}
		if request.MapReference.Set {
			value, validationErr := validateOptionalText(request.MapReference.Value, 1000, "mapReference")
			if validationErr != nil {
				return nil, validationErr
			}
			current.MapReference = value
			changedFields = append(changedFields, "mapReference")
		}
		changed := !reflect.DeepEqual(before, *current)
		if changed {
			current.UpdatedAt = s.now().UTC()
			current, err = s.spaces.Update(txCtx, current)
			if errors.Is(err, appErrors.ErrConflict) {
				return nil, adminAPIError(http.StatusConflict, "SLUG_ALREADY_EXISTS", "Já existe um Space com este slug.")
			}
			if err != nil {
				return nil, appErrors.InternalError
			}
		}
		response := appMappers.MapSpaceToResponseDTO(current)
		entityID := current.ID
		return &adminWriteOutcome[messages.SpaceResponseDTO]{Response: response, EntityType: "space", EntityRef: current.ID, EntityID: &entityID, AuditMetadata: map[string]any{"changed": changed, "fields": changedFields}, HTTPStatus: http.StatusOK}, nil
	})
}

func validateActivityEntity(activity *activityEntities.Activity) error {
	if _, err := validateAdminSlug(&activity.Slug); err != nil {
		return err
	}
	if _, err := validateAdminName(&activity.Name); err != nil {
		return err
	}
	if activity.Description != nil && len(*activity.Description) > 4000 {
		return adminAPIError(http.StatusBadRequest, "INVALID_REQUEST", "description excede o tamanho permitido.")
	}
	validKind := activity.Kind == activityEntities.KindSchedule || activity.Kind == activityEntities.KindCheckpoint || activity.Kind == activityEntities.KindChallenge || activity.Kind == activityEntities.KindCompetitive || activity.Kind == activityEntities.KindLive
	if !validKind {
		return adminAPIError(http.StatusBadRequest, "INVALID_REQUEST", "kind inválido.")
	}
	if activity.CheckInPoints < 0 || activity.MomentPoints < 0 || activity.CooldownSeconds < 0 {
		return adminAPIError(http.StatusBadRequest, "INVALID_REQUEST", "Pontos e cooldown devem ser não negativos.")
	}
	if activity.StartsAt != nil && activity.EndsAt != nil && !activity.StartsAt.Before(*activity.EndsAt) {
		return adminAPIError(http.StatusBadRequest, "INVALID_REQUEST", "startsAt deve ser anterior a endsAt.")
	}
	if activity.AllowsMoment && activity.Kind == activityEntities.KindSchedule {
		return adminAPIError(http.StatusBadRequest, "INVALID_REQUEST", "Activities schedule não permitem Moment.")
	}
	return nil
}

func (s *AdminInstallationService) resolveSpace(ctx context.Context, raw *string) (*string, error) {
	if raw == nil {
		return nil, nil
	}
	id, err := uuid.Parse(*raw)
	if err != nil {
		return nil, adminAPIError(http.StatusNotFound, "NOT_FOUND", "Space não encontrado.")
	}
	space, err := s.spaces.FindByIDForUpdate(ctx, id.String())
	if errors.Is(err, appErrors.ErrNotFound) {
		return nil, adminAPIError(http.StatusNotFound, "NOT_FOUND", "Space não encontrado.")
	}
	if err != nil {
		return nil, appErrors.InternalError
	}
	return &space.ID, nil
}

func (s *AdminInstallationService) ListActivities(ctx context.Context, filter *messages.ListAdminActivitiesFilterDTO) (*messages.PaginatedResponse[messages.AdminActivityResponseDTO], error) {
	if _, err := s.authorizeAdmin(ctx, false); err != nil {
		return nil, err
	}
	result, err := s.activities.List(ctx, filter.GetPage())
	if err != nil {
		return nil, appErrors.InternalError
	}
	items := make([]messages.AdminActivityResponseDTO, len(result.Data))
	for index := range result.Data {
		items[index] = *mapAdminActivity(&result.Data[index])
	}
	return &messages.PaginatedResponse[messages.AdminActivityResponseDTO]{Data: items, Pagination: result.Pagination}, nil
}

func (s *AdminInstallationService) CreateActivity(ctx context.Context, key string, request *messages.CreateAdminActivityRequestDTO) (*messages.AdminActivityResponseDTO, error) {
	return runAdminWrite(s, ctx, key, "admin.activity.create", request, func(txCtx context.Context, _ uint64) (*adminWriteOutcome[messages.AdminActivityResponseDTO], error) {
		if request == nil || request.Slug == nil || request.Name == nil || request.Kind == nil || request.CheckInPoints == nil || request.MomentPoints == nil || request.CooldownSeconds == nil || request.AllowsMoment == nil {
			return nil, adminAPIError(http.StatusBadRequest, "INVALID_REQUEST", "Campos obrigatórios ausentes.")
		}
		slug, err := validateAdminSlug(request.Slug)
		if err != nil {
			return nil, err
		}
		name, err := validateAdminName(request.Name)
		if err != nil {
			return nil, err
		}
		description, err := validateOptionalText(request.Description.Value, 4000, "description")
		if err != nil {
			return nil, err
		}
		spaceID, err := s.resolveSpace(txCtx, request.SpaceID.Value)
		if err != nil {
			return nil, err
		}
		kind := activityEntities.Kind(strings.TrimSpace(*request.Kind))
		now := s.now().UTC()
		initialStatus := activityEntities.StatusDraft
		if kind == activityEntities.KindSchedule || kind == activityEntities.KindCheckpoint {
			initialStatus = activityEntities.StatusActive
		}
		activity := &activityEntities.Activity{ID: uuid.NewString(), SpaceID: spaceID, Slug: slug, Name: name, Description: description, Kind: kind, Status: initialStatus, StartsAt: adminUTCTime(request.StartsAt.Value), EndsAt: adminUTCTime(request.EndsAt.Value), CheckInPoints: *request.CheckInPoints, MomentPoints: *request.MomentPoints, CooldownSeconds: *request.CooldownSeconds, AllowsMoment: *request.AllowsMoment, CreatedAt: now, UpdatedAt: now}
		if err := validateActivityEntity(activity); err != nil {
			return nil, err
		}
		created, err := s.activities.Create(txCtx, activity)
		if errors.Is(err, appErrors.ErrConflict) {
			return nil, adminAPIError(http.StatusConflict, "SLUG_ALREADY_EXISTS", "Já existe uma Activity com este slug.")
		}
		if err != nil {
			return nil, appErrors.InternalError
		}
		response := mapAdminActivity(created)
		entityID := created.ID
		return &adminWriteOutcome[messages.AdminActivityResponseDTO]{Response: response, EntityType: "activity", EntityRef: created.ID, EntityID: &entityID, AuditMetadata: map[string]any{"created": true, "status": string(initialStatus)}, HTTPStatus: http.StatusCreated}, nil
	})
}

func (s *AdminInstallationService) UpdateActivity(ctx context.Context, rawActivityID, key string, request *messages.UpdateAdminActivityRequestDTO) (*messages.AdminActivityResponseDTO, error) {
	intent := struct {
		ID      string
		Request *messages.UpdateAdminActivityRequestDTO
	}{rawActivityID, request}
	return runAdminWrite(s, ctx, key, "admin.activity.update", intent, func(txCtx context.Context, _ uint64) (*adminWriteOutcome[messages.AdminActivityResponseDTO], error) {
		activityID, err := uuid.Parse(rawActivityID)
		if err != nil {
			return nil, adminAPIError(http.StatusNotFound, "NOT_FOUND", "Activity não encontrada.")
		}
		if request == nil {
			return nil, adminAPIError(http.StatusBadRequest, "INVALID_REQUEST", "Corpo obrigatório.")
		}
		anyField := request.SpaceID.Set || request.Slug.Set || request.Name.Set || request.Description.Set || request.Kind.Set || request.StartsAt.Set || request.EndsAt.Set || request.CheckInPoints.Set || request.MomentPoints.Set || request.CooldownSeconds.Set || request.AllowsMoment.Set || request.Status.Set
		if !anyField {
			return nil, adminAPIError(http.StatusBadRequest, "INVALID_REQUEST", "Informe ao menos um campo para alteração.")
		}
		current, err := s.activities.FindByIDForUpdate(txCtx, activityID.String())
		if errors.Is(err, appErrors.ErrNotFound) {
			return nil, adminAPIError(http.StatusNotFound, "NOT_FOUND", "Activity não encontrada.")
		}
		if err != nil {
			return nil, appErrors.InternalError
		}
		configField := request.SpaceID.Set || request.Slug.Set || request.Name.Set || request.Description.Set || request.Kind.Set || request.StartsAt.Set || request.EndsAt.Set || request.CheckInPoints.Set || request.MomentPoints.Set || request.CooldownSeconds.Set || request.AllowsMoment.Set
		if current.Status == activityEntities.StatusArchived && configField {
			return nil, adminAPIError(http.StatusConflict, "ACTIVITY_STATE_CONFLICT", "Activity arquivada não pode ser reconfigurada.")
		}
		before := *current
		fields := make([]string, 0, 12)
		if request.SpaceID.Set {
			value, e := s.resolveSpace(txCtx, request.SpaceID.Value)
			if e != nil {
				return nil, e
			}
			current.SpaceID = value
			fields = append(fields, "spaceId")
		}
		if request.Slug.Set {
			value, e := validateAdminSlug(request.Slug.Value)
			if e != nil {
				return nil, e
			}
			current.Slug = value
			fields = append(fields, "slug")
		}
		if request.Name.Set {
			value, e := validateAdminName(request.Name.Value)
			if e != nil {
				return nil, e
			}
			current.Name = value
			fields = append(fields, "name")
		}
		if request.Description.Set {
			value, e := validateOptionalText(request.Description.Value, 4000, "description")
			if e != nil {
				return nil, e
			}
			current.Description = value
			fields = append(fields, "description")
		}
		if request.Kind.Set {
			if request.Kind.Value == nil {
				return nil, adminAPIError(http.StatusBadRequest, "INVALID_REQUEST", "kind não aceita null.")
			}
			current.Kind = activityEntities.Kind(strings.TrimSpace(*request.Kind.Value))
			fields = append(fields, "kind")
		}
		if request.StartsAt.Set {
			current.StartsAt = adminUTCTime(request.StartsAt.Value)
			fields = append(fields, "startsAt")
		}
		if request.EndsAt.Set {
			current.EndsAt = adminUTCTime(request.EndsAt.Value)
			fields = append(fields, "endsAt")
		}
		if request.CheckInPoints.Set {
			if request.CheckInPoints.Value == nil {
				return nil, adminAPIError(http.StatusBadRequest, "INVALID_REQUEST", "checkInPoints não aceita null.")
			}
			current.CheckInPoints = *request.CheckInPoints.Value
			fields = append(fields, "checkInPoints")
		}
		if request.MomentPoints.Set {
			if request.MomentPoints.Value == nil {
				return nil, adminAPIError(http.StatusBadRequest, "INVALID_REQUEST", "momentPoints não aceita null.")
			}
			current.MomentPoints = *request.MomentPoints.Value
			fields = append(fields, "momentPoints")
		}
		if request.CooldownSeconds.Set {
			if request.CooldownSeconds.Value == nil {
				return nil, adminAPIError(http.StatusBadRequest, "INVALID_REQUEST", "cooldownSeconds não aceita null.")
			}
			current.CooldownSeconds = *request.CooldownSeconds.Value
			fields = append(fields, "cooldownSeconds")
		}
		if request.AllowsMoment.Set {
			if request.AllowsMoment.Value == nil {
				return nil, adminAPIError(http.StatusBadRequest, "INVALID_REQUEST", "allowsMoment não aceita null.")
			}
			current.AllowsMoment = *request.AllowsMoment.Value
			fields = append(fields, "allowsMoment")
		}
		if request.Status.Set {
			if request.Status.Value == nil {
				return nil, adminAPIError(http.StatusBadRequest, "INVALID_REQUEST", "status não aceita null.")
			}
			target := activityEntities.Status(*request.Status.Value)
			allowed := false
			switch target {
			case activityEntities.StatusActive:
				allowed = current.Status == activityEntities.StatusDraft || current.Status == activityEntities.StatusPaused
			case activityEntities.StatusPaused:
				allowed = current.Status == activityEntities.StatusActive
			case activityEntities.StatusArchived:
				allowed = current.Status == activityEntities.StatusDraft || current.Status == activityEntities.StatusPaused || current.Status == activityEntities.StatusCompleted || current.Status == activityEntities.StatusArchived
			}
			if !allowed {
				return nil, adminAPIError(http.StatusConflict, "ACTIVITY_STATE_CONFLICT", "Transição de status inválida para o estado atual.")
			}
			current.Status = target
			fields = append(fields, "status")
		}
		if err := validateActivityEntity(current); err != nil {
			return nil, err
		}
		changed := !reflect.DeepEqual(before, *current)
		if changed {
			current.UpdatedAt = s.now().UTC()
			current, err = s.activities.Update(txCtx, current)
			if errors.Is(err, appErrors.ErrConflict) {
				return nil, adminAPIError(http.StatusConflict, "SLUG_ALREADY_EXISTS", "Já existe uma Activity com este slug.")
			}
			if err != nil {
				return nil, appErrors.InternalError
			}
		}
		response := mapAdminActivity(current)
		entityID := current.ID
		metadata := map[string]any{"changed": changed, "fields": fields}
		if before.Status != current.Status {
			metadata["fromStatus"] = string(before.Status)
			metadata["toStatus"] = string(current.Status)
		}
		return &adminWriteOutcome[messages.AdminActivityResponseDTO]{Response: response, EntityType: "activity", EntityRef: current.ID, EntityID: &entityID, AuditMetadata: metadata, HTTPStatus: http.StatusOK}, nil
	})
}

func (s *AdminInstallationService) ListStaff(ctx context.Context, filter *messages.ListAdminStaffFilterDTO) (*messages.PaginatedResponse[messages.AdminStaffResponseDTO], error) {
	if _, err := s.authorizeAdmin(ctx, false); err != nil {
		return nil, err
	}
	var roles []userEntities.UserRole
	var page uint64
	switch {
	case filter == nil || filter.Role == "":
		roles = []userEntities.UserRole{userEntities.RoleAdmin, userEntities.RoleEventManager}
	case filter.Role == string(userEntities.RoleAdmin):
		roles = []userEntities.UserRole{userEntities.RoleAdmin}
	case filter.Role == string(userEntities.RoleEventManager):
		roles = []userEntities.UserRole{userEntities.RoleEventManager}
	case filter.Role == string(userEntities.RoleDefault):
		roles = []userEntities.UserRole{userEntities.RoleDefault}
	default:
		return nil, adminAPIError(http.StatusBadRequest, "INVALID_REQUEST", "role deve ser DEFAULT, ADMIN ou EVENT_MANAGER.")
	}
	if filter != nil {
		page = filter.GetPage()
	}
	result, err := s.users.ListByRole(ctx, roles, page)
	if err != nil {
		return nil, appErrors.InternalError
	}
	items := make([]messages.AdminStaffResponseDTO, len(result.Data))
	for index := range result.Data {
		items[index] = mapAdminStaff(&result.Data[index])
	}
	return &messages.PaginatedResponse[messages.AdminStaffResponseDTO]{Data: items, Pagination: result.Pagination}, nil
}

func parseAdminUserID(raw string) (uint64, error) {
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		return 0, adminAPIError(http.StatusNotFound, "NOT_FOUND", "Usuário não encontrado.")
	}
	return id, nil
}

func validManagerScope(value string) bool {
	switch value {
	case "actions", "space", "pastoral_queue", "special_events":
		return true
	default:
		return false
	}
}

func sameScope(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func scopeValue(scope *string) string {
	if scope == nil {
		return ""
	}
	return *scope
}

func (s *AdminInstallationService) UpdateUserRole(ctx context.Context, rawUserID, key string, request *messages.UpdateAdminUserRoleRequestDTO) (*messages.AdminUserRoleResponseDTO, error) {
	intent := struct {
		ID      string
		Request *messages.UpdateAdminUserRoleRequestDTO
	}{rawUserID, request}
	return runAdminWrite(s, ctx, key, "admin.user.role.update", intent, func(txCtx context.Context, _ uint64) (*adminWriteOutcome[messages.AdminUserRoleResponseDTO], error) {
		if request == nil || request.Role == nil {
			return nil, adminAPIError(http.StatusBadRequest, "INVALID_REQUEST", "role é obrigatório.")
		}
		targetRole := userEntities.UserRole(*request.Role)
		if targetRole != userEntities.RoleDefault && targetRole != userEntities.RoleEventManager {
			return nil, adminAPIError(http.StatusBadRequest, "INVALID_REQUEST", "role aceita somente DEFAULT ou EVENT_MANAGER.")
		}
		if request.Scope != nil && !validManagerScope(*request.Scope) {
			return nil, adminAPIError(http.StatusBadRequest, "INVALID_REQUEST", "scope deve ser actions, space, pastoral_queue ou special_events.")
		}
		identifier, unescapeErr := url.PathUnescape(rawUserID)
		if unescapeErr != nil {
			return nil, adminAPIError(http.StatusNotFound, "NOT_FOUND", "Usuário não encontrado.")
		}
		var userID uint64
		var target *userEntities.User
		var err error
		if parsedID, parseErr := strconv.ParseUint(identifier, 10, 64); parseErr == nil && parsedID > 0 {
			userID = parsedID
			target, err = s.users.FindByIDForUpdate(txCtx, userID)
		} else if strings.Contains(identifier, "@") {
			target, err = s.users.FindByEmail(txCtx, strings.ToLower(strings.TrimSpace(identifier)))
			if target != nil {
				userID = target.ID
			}
		} else {
			return nil, adminAPIError(http.StatusNotFound, "NOT_FOUND", "Usuário não encontrado.")
		}
		if target == nil && err == nil {
			err = appErrors.ErrNotFound
		}
		if errors.Is(err, appErrors.ErrNotFound) {
			return nil, adminAPIError(http.StatusNotFound, "NOT_FOUND", "Usuário não encontrado.")
		}
		if err != nil {
			return nil, appErrors.InternalError
		}
		if target.Role != userEntities.RoleDefault && target.Role != userEntities.RoleEventManager {
			return nil, adminAPIError(http.StatusConflict, "ROLE_CHANGE_NOT_ALLOWED", "ADMIN não pode ser concedido ou removido por esta operação.")
		}
		if target.Role == userEntities.RoleEventManager && targetRole == userEntities.RoleDefault {
			count, countErr := s.activities.CountManagerAssignments(txCtx, userID)
			if countErr != nil {
				return nil, appErrors.InternalError
			}
			if count > 0 {
				return nil, adminAPIError(http.StatusConflict, "MANAGER_HAS_ASSIGNMENTS", "Remova os assignments antes de rebaixar o gestor.")
			}
		}
		fromRole := target.Role
		fromScope := target.ManagerScope
		toScope := target.ManagerScope
		if targetRole == userEntities.RoleDefault {
			toScope = nil
		} else if request.Scope != nil {
			scope := *request.Scope
			toScope = &scope
		}
		changed := fromRole != targetRole || !sameScope(fromScope, toScope)
		if changed {
			target.Role = targetRole
			target.ManagerScope = toScope
			if _, err := s.users.Update(txCtx, target); err != nil {
				return nil, appErrors.InternalError
			}
		}
		response := &messages.AdminUserRoleResponseDTO{ID: messages.Uint64StringFromUint64(userID), Role: string(targetRole), Scope: scopeValue(toScope)}
		ref := strconv.FormatUint(userID, 10)
		return &adminWriteOutcome[messages.AdminUserRoleResponseDTO]{Response: response, EntityType: "user", EntityRef: ref, AuditMetadata: map[string]any{"changed": changed, "fromRole": string(fromRole), "toRole": string(targetRole), "fromScope": scopeValue(fromScope), "toScope": scopeValue(toScope)}, HTTPStatus: http.StatusOK}, nil
	})
}

func (s *AdminInstallationService) ListManagers(ctx context.Context, rawActivityID string, filter *messages.ListAdminManagersFilterDTO) (*messages.PaginatedResponse[messages.AdminStaffResponseDTO], error) {
	if _, err := s.authorizeAdmin(ctx, false); err != nil {
		return nil, err
	}
	activityID, err := uuid.Parse(rawActivityID)
	if err != nil {
		return nil, adminAPIError(http.StatusNotFound, "NOT_FOUND", "Activity não encontrada.")
	}
	if _, err = s.activities.FindByID(ctx, activityID.String()); errors.Is(err, appErrors.ErrNotFound) {
		return nil, adminAPIError(http.StatusNotFound, "NOT_FOUND", "Activity não encontrada.")
	} else if err != nil {
		return nil, appErrors.InternalError
	}
	result, err := s.activities.ListManagers(ctx, activityID.String(), filter.GetPage())
	if err != nil {
		return nil, appErrors.InternalError
	}
	items := make([]messages.AdminStaffResponseDTO, len(result.Data))
	for index := range result.Data {
		items[index] = mapAdminStaff(&result.Data[index])
	}
	return &messages.PaginatedResponse[messages.AdminStaffResponseDTO]{Data: items, Pagination: result.Pagination}, nil
}

func assignmentReference(activityID string, userID uint64) string {
	return activityID + ":" + strconv.FormatUint(userID, 10)
}

func (s *AdminInstallationService) AssignManager(ctx context.Context, rawActivityID, rawUserID, key string) (*messages.AdminManagerAssignmentResponseDTO, error) {
	intent := struct{ ActivityID, UserID string }{rawActivityID, rawUserID}
	return runAdminWrite(s, ctx, key, "admin.activity.manager.assign", intent, func(txCtx context.Context, _ uint64) (*adminWriteOutcome[messages.AdminManagerAssignmentResponseDTO], error) {
		activityID, err := uuid.Parse(rawActivityID)
		if err != nil {
			return nil, adminAPIError(http.StatusNotFound, "NOT_FOUND", "Activity não encontrada.")
		}
		userID, err := parseAdminUserID(rawUserID)
		if err != nil {
			return nil, err
		}
		target, err := s.users.FindByIDForUpdate(txCtx, userID)
		if errors.Is(err, appErrors.ErrNotFound) {
			return nil, adminAPIError(http.StatusNotFound, "NOT_FOUND", "Usuário não encontrado.")
		}
		if err != nil {
			return nil, appErrors.InternalError
		}
		if _, err = s.activities.FindByIDForUpdate(txCtx, activityID.String()); errors.Is(err, appErrors.ErrNotFound) {
			return nil, adminAPIError(http.StatusNotFound, "NOT_FOUND", "Activity não encontrada.")
		} else if err != nil {
			return nil, appErrors.InternalError
		}
		if !target.OnboardingComplete || target.Role != userEntities.RoleEventManager {
			return nil, adminAPIError(http.StatusConflict, "MANAGER_NOT_ELIGIBLE", "Usuário deve ter onboarding completo e papel EVENT_MANAGER.")
		}
		created, err := s.activities.CreateManagerAssignment(txCtx, &activityEntities.ManagerAssignment{ActivityID: activityID.String(), UserID: userID, CreatedAt: s.now().UTC()})
		if err != nil {
			return nil, appErrors.InternalError
		}
		response := &messages.AdminManagerAssignmentResponseDTO{ActivityID: activityID.String(), UserID: messages.Uint64StringFromUint64(userID)}
		return &adminWriteOutcome[messages.AdminManagerAssignmentResponseDTO]{Response: response, EntityType: "activity_manager_assignment", EntityRef: assignmentReference(activityID.String(), userID), AuditMetadata: map[string]any{"changed": created}, HTTPStatus: http.StatusOK}, nil
	})
}

func (s *AdminInstallationService) RemoveManager(ctx context.Context, rawActivityID, rawUserID, key string) error {
	intent := struct{ ActivityID, UserID string }{rawActivityID, rawUserID}
	_, err := runAdminWrite(s, ctx, key, "admin.activity.manager.remove", intent, func(txCtx context.Context, _ uint64) (*adminWriteOutcome[struct{}], error) {
		activityID, err := uuid.Parse(rawActivityID)
		if err != nil {
			return nil, adminAPIError(http.StatusNotFound, "NOT_FOUND", "Activity não encontrada.")
		}
		userID, err := parseAdminUserID(rawUserID)
		if err != nil {
			return nil, err
		}
		if _, err = s.users.FindByIDForUpdate(txCtx, userID); errors.Is(err, appErrors.ErrNotFound) {
			return nil, adminAPIError(http.StatusNotFound, "NOT_FOUND", "Usuário não encontrado.")
		} else if err != nil {
			return nil, appErrors.InternalError
		}
		if _, err = s.activities.FindByIDForUpdate(txCtx, activityID.String()); errors.Is(err, appErrors.ErrNotFound) {
			return nil, adminAPIError(http.StatusNotFound, "NOT_FOUND", "Activity não encontrada.")
		} else if err != nil {
			return nil, appErrors.InternalError
		}
		removed, err := s.activities.DeleteManagerAssignment(txCtx, activityID.String(), userID)
		if err != nil {
			return nil, appErrors.InternalError
		}
		response := &struct{}{}
		return &adminWriteOutcome[struct{}]{Response: response, EntityType: "activity_manager_assignment", EntityRef: assignmentReference(activityID.String(), userID), AuditMetadata: map[string]any{"changed": removed}, HTTPStatus: http.StatusNoContent}, nil
	})
	return err
}
