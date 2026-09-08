package services

import (
	"context"
	"net/http"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	appInterfaces "github.com/dnjtechteam/dnj-game-api/internal/app/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	eventInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/eventsettings/interfaces"
	userEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
	userInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/user/interfaces"
)

type EventSettingsService struct {
	*BaseService
	eventSettings eventInterfaces.EventSettingsRepositoryInterface
	users         userInterfaces.UserRepositoryInterface
}

func NewEventSettingsService(
	base *BaseService,
	eventSettings eventInterfaces.EventSettingsRepositoryInterface,
	users userInterfaces.UserRepositoryInterface,
) appInterfaces.EventSettingsServiceInterface {
	return &EventSettingsService{BaseService: base, eventSettings: eventSettings, users: users}
}

func (s *EventSettingsService) requireAdmin(ctx context.Context) (*userEntities.User, error) {
	id, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	user, err := s.users.FindByID(ctx, id)
	if err != nil {
		return nil, appErrors.InternalError
	}
	if user == nil {
		return nil, appErrors.NewAPIServiceError(http.StatusUnauthorized, "UNAUTHENTICATED", "Autenticação necessária.", nil)
	}
	if user.Role != userEntities.RoleAdmin {
		return nil, appErrors.NewAPIServiceError(http.StatusForbidden, "FORBIDDEN", "Operação restrita a ADMIN.", nil)
	}
	return user, nil
}

func (s *EventSettingsService) GetScoringStatus(ctx context.Context) (*messages.ScoringStatusResponseDTO, error) {
	settings, err := s.eventSettings.Get(ctx)
	if err != nil {
		return nil, appErrors.InternalError
	}
	return &messages.ScoringStatusResponseDTO{
		ScoringClosed: settings.ScoringClosed,
		ClosedAt:      settings.ClosedAt,
		AuditNote:     settings.AuditNote,
	}, nil
}

func (s *EventSettingsService) CloseScoring(ctx context.Context, auditNote string) (*messages.ScoringStatusResponseDTO, error) {
	admin, err := s.requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	settings, err := s.eventSettings.Get(ctx)
	if err != nil {
		return nil, appErrors.InternalError
	}
	if settings.ScoringClosed {
		return nil, appErrors.NewAPIServiceError(http.StatusConflict, "SCORING_ALREADY_CLOSED", "A pontuação já está fechada.", nil)
	}
	if err := s.eventSettings.SetScoringClosed(ctx, true, &admin.ID, auditNote); err != nil {
		return nil, appErrors.InternalError
	}
	return s.GetScoringStatus(ctx)
}

func (s *EventSettingsService) OpenScoring(ctx context.Context, auditNote string) (*messages.ScoringStatusResponseDTO, error) {
	admin, err := s.requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	settings, err := s.eventSettings.Get(ctx)
	if err != nil {
		return nil, appErrors.InternalError
	}
	if !settings.ScoringClosed {
		return nil, appErrors.NewAPIServiceError(http.StatusConflict, "SCORING_ALREADY_OPEN", "A pontuação já está aberta.", nil)
	}
	if err := s.eventSettings.SetScoringClosed(ctx, false, &admin.ID, auditNote); err != nil {
		return nil, appErrors.InternalError
	}
	return s.GetScoringStatus(ctx)
}

func (s *EventSettingsService) IsScoringClosed(ctx context.Context) (bool, error) {
	settings, err := s.eventSettings.Get(ctx)
	if err != nil {
		return false, err
	}
	return settings.ScoringClosed, nil
}
