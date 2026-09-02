package services

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	appInterfaces "github.com/dnjtechteam/dnj-game-api/internal/app/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	activityEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/activity/entities"
	activityInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/activity/interfaces"
	gameEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/game/entities"
	gameInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/game/interfaces"
	notificationEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/notification/entities"
	notificationInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/notification/interfaces"
	specialEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/specialevent/entities"
	specialInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/specialevent/interfaces"
	userEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
	userInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/user/interfaces"
	infraCommon "github.com/dnjtechteam/dnj-game-api/internal/infrastructure/common"
	"github.com/google/uuid"
)

const specialEventTeaser = 15 * time.Second

type SpecialEventService struct {
	*BaseService
	events        specialInterfaces.Repository
	activities    activityInterfaces.ActivityRepositoryInterface
	games         gameInterfaces.GameRepositoryInterface
	users         userInterfaces.UserRepositoryInterface
	notifications notificationInterfaces.Repository
	now           func() time.Time
	secret        func() string
}

func NewSpecialEventService(base *BaseService, events specialInterfaces.Repository, activities activityInterfaces.ActivityRepositoryInterface, games gameInterfaces.GameRepositoryInterface, users userInterfaces.UserRepositoryInterface, notifications notificationInterfaces.Repository) appInterfaces.SpecialEventServiceInterface {
	return &SpecialEventService{BaseService: base, events: events, activities: activities, games: games, users: users, notifications: notifications, now: time.Now, secret: func() string { return os.Getenv("DOCUMENT_HMAC_SECRET") }}
}

func (s *SpecialEventService) announceActivation(ctx context.Context, event *specialEntities.Event, now time.Time) error {
	recipients, err := s.notifications.ResolveAnnouncementRecipients(ctx, nil)
	if err != nil {
		return appErrors.InternalError
	}
	body := event.Title
	if event.Description != nil && strings.TrimSpace(*event.Description) != "" {
		body = strings.TrimSpace(*event.Description)
	}
	rows := make([]*notificationEntities.Notification, len(recipients))
	for i, userID := range recipients {
		eventID := event.ID
		rows[i] = &notificationEntities.Notification{ID: uuid.NewString(), UserID: userID, Category: notificationEntities.CategorySpecialEvent, State: notificationEntities.StateUnread, Title: "Desafio Especial disponível", Body: body, SourceType: "special_event", SourceID: &eventID, CreatedAt: now}
	}
	if err := s.notifications.CreateBroadcast(ctx, rows); err != nil {
		return appErrors.InternalError
	}
	return nil
}

func specialError(status int, code, message string) error {
	return appErrors.NewAPIServiceError(status, code, message, nil)
}
func (s *SpecialEventService) manager(ctx context.Context) (*userEntities.User, bool, error) {
	id := infraCommon.ExtractUserIdFromContext(ctx)
	if id == 0 {
		return nil, false, specialError(http.StatusUnauthorized, "UNAUTHENTICATED", "Autenticação necessária.")
	}
	actor, err := s.users.FindByID(ctx, id)
	if errors.Is(err, appErrors.ErrNotFound) {
		return nil, false, specialError(http.StatusUnauthorized, "UNAUTHENTICATED", "Autenticação necessária.")
	}
	if err != nil {
		return nil, false, appErrors.InternalError
	}
	if actor.Role != userEntities.RoleAdmin && actor.Role != userEntities.RoleEventManager {
		return nil, false, specialError(http.StatusForbidden, "FORBIDDEN", "Operação não permitida para este papel.")
	}
	if actor.Role != userEntities.RoleAdmin && (actor.ManagerScope == nil || *actor.ManagerScope != "special_events") {
		return nil, false, specialError(http.StatusForbidden, "FORBIDDEN", "Esta operação exige o escopo de Eventos especiais.")
	}
	return actor, actor.Role == userEntities.RoleAdmin, nil
}
func specialDTO(event *specialEntities.Event) *messages.ManagerSpecialEventDTO {
	return &messages.ManagerSpecialEventDTO{ID: event.ID, Title: event.Title, Description: event.Description, Points: event.Points, Status: string(event.Status), ExpiresAt: event.EndsAt, QRAvailableAt: event.TeaserAt}
}
func validTargets(targets []string) bool {
	if len(targets) == 0 {
		return false
	}
	seen := map[string]bool{}
	for _, t := range targets {
		if t != "app" && t != "tv" && t != "screen" {
			return false
		}
		if seen[t] {
			return false
		}
		seen[t] = true
	}
	return true
}
func (s *SpecialEventService) ListManager(ctx context.Context) (*messages.ManagerSpecialEventsResponseDTO, error) {
	actor, global, err := s.manager(ctx)
	if err != nil {
		return nil, err
	}
	events, err := s.events.ListForManager(ctx, actor.ID, global)
	if err != nil {
		return nil, appErrors.InternalError
	}
	result := &messages.ManagerSpecialEventsResponseDTO{Events: make([]messages.ManagerSpecialEventDTO, len(events))}
	for i := range events {
		result.Events[i] = *specialDTO(&events[i])
	}
	return result, nil
}
func (s *SpecialEventService) Create(ctx context.Context, request *messages.CreateSpecialEventRequestDTO) (*messages.ManagerSpecialEventDTO, error) {
	if request == nil || strings.TrimSpace(request.Title) == "" || len(strings.TrimSpace(request.Title)) > 100 || request.Points < 0 || request.DurationMinutes < 1 || request.DurationMinutes > 180 || !validTargets(request.Targets) {
		return nil, specialError(http.StatusBadRequest, "INVALID_REQUEST", "Informe título, duração, pontos e destinos válidos.")
	}
	var result *messages.ManagerSpecialEventDTO
	err := s.WithTransaction(ctx, func(tx context.Context) error {
		actor, _, err := s.manager(tx)
		if err != nil {
			return err
		}
		now := s.now().UTC()
		id := uuid.NewString()
		ends := now.Add(time.Duration(request.DurationMinutes) * time.Minute)
		title := strings.TrimSpace(request.Title)
		var description *string
		if text := strings.TrimSpace(request.Description); text != "" {
			description = &text
		}
		activity := &activityEntities.Activity{ID: id, Slug: "special-" + id[:8], Name: title, Description: description, Kind: activityEntities.KindLive, Status: activityEntities.StatusActive, StartsAt: &now, EndsAt: &ends, CheckInPoints: request.Points, AllowsMoment: false, CreatedAt: now, UpdatedAt: now}
		if _, err = s.activities.Create(tx, activity); err != nil {
			return appErrors.InternalError
		}
		if _, err = s.activities.CreateManagerAssignment(tx, &activityEntities.ManagerAssignment{ActivityID: id, UserID: actor.ID, CreatedAt: now}); err != nil {
			return appErrors.InternalError
		}
		event := &specialEntities.Event{ID: id, ActivityID: id, Title: title, Description: description, Points: request.Points, DurationMinutes: request.DurationMinutes, Targets: request.Targets, Status: specialEntities.StatusDraft, EndsAt: ends, CreatedBy: actor.ID, CreatedAt: now, UpdatedAt: now}
		if err = s.events.Create(tx, event); err != nil {
			return appErrors.InternalError
		}
		result = specialDTO(event)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
func (s *SpecialEventService) Teaser(ctx context.Context, eventID string) (*messages.ManagerSpecialEventDTO, error) {
	var result *messages.ManagerSpecialEventDTO
	err := s.WithTransaction(ctx, func(tx context.Context) error {
		actor, global, err := s.manager(tx)
		if err != nil {
			return err
		}
		event, err := s.events.FindForManager(tx, eventID, actor.ID, global, true)
		if errors.Is(err, appErrors.ErrNotFound) {
			return specialError(http.StatusNotFound, "NOT_FOUND", "Evento especial não encontrado.")
		}
		if err != nil {
			return appErrors.InternalError
		}
		if event.Status != specialEntities.StatusDraft {
			return specialError(http.StatusConflict, "EVENT_STATE_CONFLICT", "Este evento não está pronto para o teaser.")
		}
		now := s.now().UTC()
		event.Status = specialEntities.StatusTeaser
		event.TeaserAt = &now
		event.UpdatedAt = now
		if err = s.events.Save(tx, event); err != nil {
			return appErrors.InternalError
		}
		result = specialDTO(event)
		return nil
	})
	return result, err
}
func (s *SpecialEventService) qrToken(qrID string) string {
	mac := hmac.New(sha256.New, []byte(s.secret()))
	_, _ = mac.Write([]byte("dnj-v2-activity-run-qr\x00" + qrID))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
func (s *SpecialEventService) qrHash(token string) string {
	mac := hmac.New(sha256.New, []byte(s.secret()))
	_, _ = mac.Write([]byte("dnj-v2-activity-run-qr-hash\x00" + token))
	return hex.EncodeToString(mac.Sum(nil))
}
func (s *SpecialEventService) ReleaseQR(ctx context.Context, eventID string) (*messages.SpecialEventQRResponseDTO, error) {
	if strings.TrimSpace(s.secret()) == "" {
		return nil, appErrors.InternalError
	}
	var result *messages.SpecialEventQRResponseDTO
	err := s.WithTransaction(ctx, func(tx context.Context) error {
		actor, global, err := s.manager(tx)
		if err != nil {
			return err
		}
		event, err := s.events.FindForManager(tx, eventID, actor.ID, global, true)
		if errors.Is(err, appErrors.ErrNotFound) {
			return specialError(http.StatusNotFound, "NOT_FOUND", "Evento especial não encontrado.")
		}
		if err != nil {
			return appErrors.InternalError
		}
		now := s.now().UTC()
		if event.Status == specialEntities.StatusTeaser && event.TeaserAt != nil && now.Before(event.TeaserAt.Add(specialEventTeaser)) {
			return specialError(http.StatusConflict, "TEASER_IN_PROGRESS", "Aguarde o teaser terminar para liberar o QR.")
		}
		if event.Status != specialEntities.StatusTeaser && event.Status != specialEntities.StatusActive {
			return specialError(http.StatusConflict, "EVENT_STATE_CONFLICT", "Este evento não pode liberar QR.")
		}
		activating := event.Status != specialEntities.StatusActive
		var runID string
		if event.ActivityRunID == nil {
			run := &gameEntities.ActivityRun{ID: uuid.NewString(), ActivityID: event.ActivityID, StartedBy: actor.ID, Status: gameEntities.RunStatusDraft, PointRules: gameEntities.DefaultPointRules(), CreatedAt: now, UpdatedAt: now}
			if _, err = s.games.CreateRun(tx, run); err != nil {
				return appErrors.InternalError
			}
			runID = run.ID
			event.ActivityRunID = &runID
		} else {
			runID = *event.ActivityRunID
			if err = s.games.DisableActiveQR(tx, runID, now); err != nil {
				return appErrors.InternalError
			}
		}
		qrID := uuid.NewString()
		token := s.qrToken(qrID)
		expires := event.EndsAt
		if _, err = s.games.CreateQR(tx, &gameEntities.QRCode{ID: qrID, ActivityID: event.ActivityID, ActivityRunID: runID, TokenHash: s.qrHash(token), ExpiresAt: expires, Status: gameEntities.QRCodeStatusActive, CreatedAt: now, UpdatedAt: now}); err != nil {
			return appErrors.InternalError
		}
		event.Status = specialEntities.StatusActive
		event.QRToken = &token
		event.QRExpiresAt = &expires
		event.UpdatedAt = now
		if err = s.events.Save(tx, event); err != nil {
			return appErrors.InternalError
		}
		if activating {
			if err = s.announceActivation(tx, event, now); err != nil {
				return err
			}
		}
		result = &messages.SpecialEventQRResponseDTO{QRToken: token, ExpiresAt: expires}
		return nil
	})
	return result, err
}
func (s *SpecialEventService) Close(ctx context.Context, eventID string) error {
	return s.WithTransaction(ctx, func(tx context.Context) error {
		actor, global, err := s.manager(tx)
		if err != nil {
			return err
		}
		event, err := s.events.FindForManager(tx, eventID, actor.ID, global, true)
		if errors.Is(err, appErrors.ErrNotFound) {
			return specialError(http.StatusNotFound, "NOT_FOUND", "Evento especial não encontrado.")
		}
		if err != nil {
			return appErrors.InternalError
		}
		now := s.now().UTC()
		if event.ActivityRunID != nil {
			_ = s.games.DisableActiveQR(tx, *event.ActivityRunID, now)
			_ = s.games.TransitionRun(tx, *event.ActivityRunID, gameEntities.RunStatusDraft, gameEntities.RunStatusCancelled, nil, &now, now)
			_ = s.games.CompleteParticipationStates(tx, *event.ActivityRunID, gameEntities.ParticipationStatusCancelled)
		}
		event.Status = specialEntities.StatusClosed
		event.QRToken = nil
		event.QRExpiresAt = nil
		event.UpdatedAt = now
		if err = s.events.Save(tx, event); err != nil {
			return appErrors.InternalError
		}
		return nil
	})
}
func (s *SpecialEventService) Active(ctx context.Context, target string) (*messages.ActiveSpecialEventResponseDTO, error) {
	if target != "app" {
		return nil, specialError(http.StatusBadRequest, "INVALID_REQUEST", "Destino inválido.")
	}
	event, err := s.events.FindVisible(ctx, target, s.now().UTC())
	if errors.Is(err, appErrors.ErrNotFound) {
		return &messages.ActiveSpecialEventResponseDTO{Event: nil, MomentChallenge: nil}, nil
	}
	if err != nil {
		return nil, appErrors.InternalError
	}
	start := event.CreatedAt
	if event.TeaserAt != nil {
		start = *event.TeaserAt
	}
	var ready *time.Time
	if event.Status == specialEntities.StatusTeaser && event.TeaserAt != nil {
		at := event.TeaserAt.Add(specialEventTeaser)
		ready = &at
	}
	return &messages.ActiveSpecialEventResponseDTO{Event: &messages.ActiveSpecialEventDTO{ID: event.ID, Title: event.Title, Status: string(event.Status), StartsAt: start, EndsAt: event.EndsAt, TeaserSeconds: 15, Points: event.Points, QRAvailableAt: ready, QRToken: event.QRToken}, MomentChallenge: nil}, nil
}
func (s *SpecialEventService) Display(ctx context.Context, target string) (*messages.LiveDisplaySpecialEventDTO, error) {
	if target != "tv" && target != "screen" {
		return nil, specialError(http.StatusBadRequest, "INVALID_REQUEST", "Destino inválido.")
	}
	event, err := s.events.FindVisible(ctx, target, s.now().UTC())
	if errors.Is(err, appErrors.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, appErrors.InternalError
	}
	var ready *time.Time
	if event.Status == specialEntities.StatusTeaser && event.TeaserAt != nil {
		at := event.TeaserAt.Add(specialEventTeaser)
		ready = &at
	}
	return &messages.LiveDisplaySpecialEventDTO{ID: event.ID, Title: event.Title, Status: string(event.Status), Points: event.Points, EndsAt: event.EndsAt, ReadyAt: ready, QRToken: event.QRToken}, nil
}
