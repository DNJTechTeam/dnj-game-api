package services

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"time"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	appInterfaces "github.com/dnjtechteam/dnj-game-api/internal/app/interfaces"
	appMappers "github.com/dnjtechteam/dnj-game-api/internal/app/mappers"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	activityEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/activity/entities"
	activityInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/activity/interfaces"
	spaceInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/space/interfaces"
	"github.com/google/uuid"
)

const upcomingWindow = 15 * time.Minute

var publicSectorSlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type ContentService struct {
	activities activityInterfaces.ActivityRepositoryInterface
	spaces     spaceInterfaces.SpaceRepositoryInterface
	now        func() time.Time
}

func NewContentService(activities activityInterfaces.ActivityRepositoryInterface, spaces spaceInterfaces.SpaceRepositoryInterface) appInterfaces.ContentServiceInterface {
	return &ContentService{activities: activities, spaces: spaces, now: time.Now}
}

func contentError(status int, code, message string) error {
	return appErrors.NewAPIServiceError(status, code, message, nil)
}

func deriveActivityState(activity *activityEntities.Activity, generatedAt time.Time) *string {
	if activity == nil || activity.StartsAt == nil || activity.EndsAt == nil || !activity.StartsAt.Before(*activity.EndsAt) {
		return nil
	}
	now := generatedAt.UTC()
	startsAt := activity.StartsAt.UTC()
	endsAt := activity.EndsAt.UTC()
	state := "scheduled"
	if activity.Status == activityEntities.StatusCompleted || !now.Before(endsAt) {
		state = "ended"
	} else if activity.Status == activityEntities.StatusActive && !now.Before(startsAt) && now.Before(endsAt) {
		state = "live"
	} else if now.Before(startsAt) && startsAt.Sub(now) <= upcomingWindow {
		state = "upcoming"
	}
	return &state
}

func (s *ContentService) Schedule(ctx context.Context, filter *messages.ListScheduleFilterDTO) (*messages.ScheduleResponseDTO, error) {
	if filter == nil || (filter.View != "" && filter.View != "home") {
		return nil, contentError(http.StatusBadRequest, "INVALID_REQUEST", "view aceita somente home ou ausência.")
	}
	if filter.Sector != "" && (len(filter.Sector) > 120 || !publicSectorSlugPattern.MatchString(filter.Sector)) {
		return nil, contentError(http.StatusBadRequest, "INVALID_REQUEST", "sector deve ser um slug válido de Space.")
	}
	generatedAt := s.now().UTC()
	limit := 100
	if filter.View == "home" {
		limit = 0
	}
	rows, err := s.activities.ListSchedule(ctx, filter.Sector, limit)
	if err != nil {
		return nil, appErrors.InternalError
	}
	items := make([]messages.ScheduleItemResponseDTO, 0, len(rows))
	if filter.View != "home" {
		for index := range rows {
			state := deriveActivityState(&rows[index].Activity, generatedAt)
			items = append(items, appMappers.MapScheduleItemToResponseDTO(&rows[index], *state))
		}
		return &messages.ScheduleResponseDTO{Items: items, GeneratedAt: generatedAt}, nil
	}
	live := make([]messages.ScheduleItemResponseDTO, 0)
	next := make([]messages.ScheduleItemResponseDTO, 0, 3)
	for index := range rows {
		state := deriveActivityState(&rows[index].Activity, generatedAt)
		if state == nil {
			continue
		}
		item := appMappers.MapScheduleItemToResponseDTO(&rows[index], *state)
		if *state == "live" {
			live = append(live, item)
		} else if len(next) < 3 && *state != "ended" && rows[index].Activity.StartsAt.After(generatedAt) {
			next = append(next, item)
		}
	}
	items = append(live, next...)
	return &messages.ScheduleResponseDTO{Items: items, GeneratedAt: generatedAt}, nil
}

func parsePublicKind(raw string) (*activityEntities.Kind, error) {
	if raw == "" {
		return nil, nil
	}
	kind := activityEntities.Kind(raw)
	switch kind {
	case activityEntities.KindSchedule, activityEntities.KindCheckpoint, activityEntities.KindChallenge, activityEntities.KindCompetitive, activityEntities.KindLive:
		return &kind, nil
	default:
		return nil, contentError(http.StatusBadRequest, "INVALID_REQUEST", "kind inválido.")
	}
}

func (s *ContentService) ListActivities(ctx context.Context, filter *messages.ListPublicActivitiesFilterDTO) (*messages.PaginatedResponse[messages.PublicActivityResponseDTO], error) {
	if filter == nil {
		return nil, contentError(http.StatusBadRequest, "INVALID_REQUEST", "Filtros inválidos.")
	}
	kind, err := parsePublicKind(filter.Kind)
	if err != nil {
		return nil, err
	}
	var spaceID *string
	if filter.SpaceID != "" {
		parsed, parseErr := uuid.Parse(filter.SpaceID)
		if parseErr != nil {
			return nil, contentError(http.StatusBadRequest, "INVALID_REQUEST", "spaceId deve ser um UUID válido.")
		}
		normalized := parsed.String()
		if _, findErr := s.spaces.FindByID(ctx, normalized); errors.Is(findErr, appErrors.ErrNotFound) {
			return nil, contentError(http.StatusNotFound, "NOT_FOUND", "Space não encontrado.")
		} else if findErr != nil {
			return nil, appErrors.InternalError
		}
		spaceID = &normalized
	}
	generatedAt := s.now().UTC()
	result, err := s.activities.ListPublic(ctx, kind, spaceID, generatedAt, filter.GetPage())
	if err != nil {
		return nil, appErrors.InternalError
	}
	data := make([]messages.PublicActivityResponseDTO, len(result.Data))
	for index := range result.Data {
		data[index] = *appMappers.MapPublicActivityToResponseDTO(&result.Data[index], deriveActivityState(&result.Data[index].Activity, generatedAt))
	}
	return &messages.PaginatedResponse[messages.PublicActivityResponseDTO]{Data: data, Pagination: result.Pagination}, nil
}

func (s *ContentService) GetActivity(ctx context.Context, rawActivityID string) (*messages.PublicActivityResponseDTO, error) {
	activityID, err := uuid.Parse(rawActivityID)
	if err != nil {
		return nil, contentError(http.StatusNotFound, "NOT_FOUND", "Atividade não encontrada.")
	}
	generatedAt := s.now().UTC()
	item, err := s.activities.FindPublicByID(ctx, activityID.String(), generatedAt)
	if errors.Is(err, appErrors.ErrNotFound) {
		return nil, contentError(http.StatusNotFound, "NOT_FOUND", "Atividade não encontrada.")
	}
	if err != nil {
		return nil, appErrors.InternalError
	}
	return appMappers.MapPublicActivityToResponseDTO(item, deriveActivityState(&item.Activity, generatedAt)), nil
}
