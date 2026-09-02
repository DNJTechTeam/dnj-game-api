package services

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	appInterfaces "github.com/dnjtechteam/dnj-game-api/internal/app/interfaces"
	appMappers "github.com/dnjtechteam/dnj-game-api/internal/app/mappers"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	notificationEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/notification/entities"
	notificationInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/notification/interfaces"
	userInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/user/interfaces"
	"github.com/google/uuid"
)

type NotificationService struct {
	*BaseService
	notifications notificationInterfaces.Repository
	users         userInterfaces.UserRepositoryInterface
	now           func() time.Time
}

func NewNotificationService(
	base *BaseService,
	notifications notificationInterfaces.Repository,
	users userInterfaces.UserRepositoryInterface,
) appInterfaces.NotificationServiceInterface {
	return &NotificationService{
		BaseService:   base,
		notifications: notifications,
		users:         users,
		now:           time.Now,
	}
}

func findNotificationOperation(
	ctx context.Context,
	repository notificationInterfaces.Repository,
	actorID uint64,
	key string,
	operation string,
	fingerprint string,
) (*notificationEntities.Operation, error) {
	prior, err := repository.FindOperation(ctx, actorID, key)
	if err == nil {
		if prior.Operation != operation || prior.IntentHash != fingerprint {
			return nil, mediaMomentError(
				http.StatusConflict,
				"IDEMPOTENCY_KEY_REUSED",
				"Idempotency-Key já foi usada em outra intenção.",
			)
		}
		return prior, nil
	}
	if !errors.Is(err, appErrors.ErrNotFound) {
		return nil, appErrors.InternalError
	}
	return nil, nil
}

func createNotificationOperation(
	ctx context.Context,
	repository notificationInterfaces.Repository,
	operation *notificationEntities.Operation,
) error {
	err := repository.CreateOperation(ctx, operation)
	if errors.Is(err, appErrors.ErrConflict) {
		return errIdempotencyRace
	}
	if err != nil {
		return appErrors.InternalError
	}
	return nil
}

func (s *NotificationService) GetPreferences(ctx context.Context) (*messages.NotificationPreferencesResponseDTO, error) {
	actor, err := requireDefaultActor(ctx, s.users, false)
	if err != nil {
		return nil, err
	}
	prefs, err := s.notifications.FindPreferences(ctx, actor.ID)
	if errors.Is(err, appErrors.ErrNotFound) {
		prefs = &notificationEntities.Preferences{
			UserID:              actor.ID,
			PointsEnabled:       true,
			AnnouncementEnabled: true,
			UpdatedAt:           utcNow(s.now),
		}
	} else if err != nil {
		return nil, appErrors.InternalError
	}
	return appMappers.MapNotificationPreferencesToResponseDTO(prefs), nil
}

func (s *NotificationService) UpdatePreferences(
	ctx context.Context,
	rawKey string,
	request *messages.UpdateNotificationPreferencesRequestDTO,
) (*messages.NotificationPreferencesResponseDTO, error) {
	key, err := parseIdempotencyKey(rawKey)
	if err != nil {
		return nil, err
	}
	if request == nil {
		return nil, mediaMomentError(http.StatusBadRequest, "INVALID_REQUEST", "Corpo inválido.")
	}
	actor, err := requireDefaultActor(ctx, s.users, false)
	if err != nil {
		return nil, err
	}
	operation := "notifications.preferences.update"
	fingerprint := intentHash(operation, request)
	now := utcNow(s.now)
	var response *messages.NotificationPreferencesResponseDTO

	err = s.WithTransaction(ctx, func(tx context.Context) error {
		prior, priorErr := findNotificationOperation(tx, s.notifications, actor.ID, key, operation, fingerprint)
		if priorErr != nil {
			return priorErr
		}
		if _, authErr := requireDefaultActor(tx, s.users, true); authErr != nil {
			return authErr
		}
		if prior != nil {
			prefs, findErr := s.notifications.FindPreferences(tx, actor.ID)
			if findErr != nil {
				return appErrors.InternalError
			}
			response = appMappers.MapNotificationPreferencesToResponseDTO(prefs)
			return nil
		}

		pointsEnabled, announcementEnabled := true, true
		existing, findErr := s.notifications.FindPreferences(tx, actor.ID)
		if findErr == nil {
			pointsEnabled = existing.PointsEnabled
			announcementEnabled = existing.AnnouncementEnabled
		} else if !errors.Is(findErr, appErrors.ErrNotFound) {
			return appErrors.InternalError
		}
		if request.PointsEnabled != nil {
			pointsEnabled = *request.PointsEnabled
		}
		if request.AnnouncementEnabled != nil {
			announcementEnabled = *request.AnnouncementEnabled
		}

		updated, upsertErr := s.notifications.UpsertPreferences(tx, &notificationEntities.Preferences{
			UserID:              actor.ID,
			PointsEnabled:       pointsEnabled,
			AnnouncementEnabled: announcementEnabled,
			UpdatedAt:           now,
		})
		if upsertErr != nil {
			return appErrors.InternalError
		}

		completedAt := now
		if createErr := createNotificationOperation(tx, s.notifications, &notificationEntities.Operation{
			ID:               uuid.NewString(),
			ActorUserID:      actor.ID,
			IdempotencyKey:   key,
			Operation:        operation,
			IntentHash:       fingerprint,
			State:            "completed",
			ResponseSnapshot: []byte(`{}`),
			HTTPStatus:       http.StatusOK,
			CreatedAt:        now,
			CompletedAt:      &completedAt,
		}); createErr != nil {
			return createErr
		}
		response = appMappers.MapNotificationPreferencesToResponseDTO(updated)
		return nil
	})
	if errors.Is(err, errIdempotencyRace) {
		return s.UpdatePreferences(ctx, rawKey, request)
	}
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (s *NotificationService) List(
	ctx context.Context,
	filter *messages.ListNotificationsFilterDTO,
) (*messages.NotificationListResponseDTO, error) {
	actor, err := requireDefaultActor(ctx, s.users, false)
	if err != nil {
		return nil, err
	}
	if filter == nil {
		filter = &messages.ListNotificationsFilterDTO{}
	}
	result, err := s.notifications.List(ctx, actor.ID, filter.GetPage())
	if err != nil {
		return nil, appErrors.InternalError
	}
	unread, err := s.notifications.CountUnread(ctx, actor.ID)
	if err != nil {
		return nil, appErrors.InternalError
	}
	return &messages.NotificationListResponseDTO{
		Data:        appMappers.MapNotificationsToResponseDTOs(result.Data),
		Pagination:  result.Pagination,
		UnreadCount: messages.Uint64StringFromUint64(unread),
	}, nil
}

func (s *NotificationService) MarkRead(
	ctx context.Context,
	rawNotificationID string,
	rawKey string,
) (*messages.NotificationResponseDTO, error) {
	notificationID, err := uuid.Parse(rawNotificationID)
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
	operation := "notifications.read"
	fingerprint := intentHash(operation, struct {
		NotificationID string `json:"notificationId"`
	}{NotificationID: notificationID.String()})
	now := utcNow(s.now)
	var response *messages.NotificationResponseDTO

	err = s.WithTransaction(ctx, func(tx context.Context) error {
		prior, priorErr := findNotificationOperation(tx, s.notifications, actor.ID, key, operation, fingerprint)
		if priorErr != nil {
			return priorErr
		}
		if _, authErr := requireDefaultActor(tx, s.users, true); authErr != nil {
			return authErr
		}
		notification, findErr := s.notifications.FindByIDAndUser(tx, notificationID.String(), actor.ID)
		if errors.Is(findErr, appErrors.ErrNotFound) {
			return notFoundError()
		}
		if findErr != nil {
			return appErrors.InternalError
		}
		if prior != nil {
			response = appMappers.MapNotificationToResponseDTO(notification)
			return nil
		}

		updated, markErr := s.notifications.MarkRead(tx, notification.ID, actor.ID, now)
		if markErr != nil {
			return appErrors.InternalError
		}

		completedAt := now
		resourceRef := notification.ID
		if createErr := createNotificationOperation(tx, s.notifications, &notificationEntities.Operation{
			ID:               uuid.NewString(),
			ActorUserID:      actor.ID,
			IdempotencyKey:   key,
			Operation:        operation,
			ResourceRef:      &resourceRef,
			IntentHash:       fingerprint,
			State:            "completed",
			ResultRef:        &resourceRef,
			ResponseSnapshot: []byte(`{}`),
			HTTPStatus:       http.StatusOK,
			CreatedAt:        now,
			CompletedAt:      &completedAt,
		}); createErr != nil {
			return createErr
		}
		response = appMappers.MapNotificationToResponseDTO(updated)
		return nil
	})
	if errors.Is(err, errIdempotencyRace) {
		return s.MarkRead(ctx, rawNotificationID, rawKey)
	}
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (s *NotificationService) AdminSend(
	ctx context.Context,
	rawKey string,
	request *messages.AdminSendNotificationRequestDTO,
) (*messages.AdminSendNotificationResponseDTO, error) {
	key, err := parseIdempotencyKey(rawKey)
	if err != nil {
		return nil, err
	}
	if request == nil || strings.TrimSpace(request.Title) == "" || strings.TrimSpace(request.Body) == "" {
		return nil, mediaMomentError(http.StatusBadRequest, "INVALID_REQUEST", "title e body são obrigatórios.")
	}
	actor, err := requireAdminActor(ctx, s.users)
	if err != nil {
		return nil, err
	}
	operation := "admin.notifications.send"
	fingerprint := intentHash(operation, request)
	now := utcNow(s.now)
	var response *messages.AdminSendNotificationResponseDTO

	err = s.WithTransaction(ctx, func(tx context.Context) error {
		prior, priorErr := findNotificationOperation(tx, s.notifications, actor.ID, key, operation, fingerprint)
		if priorErr != nil {
			return priorErr
		}
		if _, authErr := requireAdminActor(tx, s.users); authErr != nil {
			return authErr
		}
		if prior != nil {
			count := 0
			if prior.ResultCount != nil {
				count = *prior.ResultCount
			}
			response = &messages.AdminSendNotificationResponseDTO{
				RecipientCount: messages.Uint64StringFromUint64(uint64(count)),
			}
			return nil
		}

		explicit := make([]uint64, len(request.TargetUserIds))
		for i, id := range request.TargetUserIds {
			explicit[i] = uint64(id)
		}
		recipients, resolveErr := s.notifications.ResolveAnnouncementRecipients(tx, explicit)
		if resolveErr != nil {
			return appErrors.InternalError
		}

		batchID := uuid.NewString()
		rows := make([]*notificationEntities.Notification, len(recipients))
		for i, userID := range recipients {
			rows[i] = &notificationEntities.Notification{
				ID:         uuid.NewString(),
				UserID:     userID,
				Category:   notificationEntities.CategoryAnnouncement,
				State:      notificationEntities.StateUnread,
				Title:      request.Title,
				Body:       request.Body,
				SourceType: "admin_broadcast",
				SourceID:   &batchID,
				CreatedAt:  now,
			}
			if request.Urgent {
				rows[i].Metadata = []byte(`{"urgent":true}`)
			}
		}
		if createErr := s.notifications.CreateBroadcast(tx, rows); createErr != nil {
			return appErrors.InternalError
		}

		count := len(recipients)
		completedAt := now
		if createErr := createNotificationOperation(tx, s.notifications, &notificationEntities.Operation{
			ID:               uuid.NewString(),
			ActorUserID:      actor.ID,
			IdempotencyKey:   key,
			Operation:        operation,
			ResourceRef:      &batchID,
			IntentHash:       fingerprint,
			State:            "completed",
			ResultRef:        &batchID,
			ResultCount:      &count,
			ResponseSnapshot: []byte(`{}`),
			HTTPStatus:       http.StatusCreated,
			CreatedAt:        now,
			CompletedAt:      &completedAt,
		}); createErr != nil {
			return createErr
		}
		response = &messages.AdminSendNotificationResponseDTO{RecipientCount: messages.Uint64StringFromUint64(uint64(count))}
		return nil
	})
	if errors.Is(err, errIdempotencyRace) {
		return s.AdminSend(ctx, rawKey, request)
	}
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (s *NotificationService) GetPushConfig(ctx context.Context) (*messages.PushConfigResponseDTO, error) {
	if _, err := requireDefaultActor(ctx, s.users, false); err != nil {
		return nil, err
	}
	key := strings.TrimSpace(os.Getenv("VAPID_PUBLIC_KEY"))
	if key == "" {
		return nil, mediaMomentError(http.StatusServiceUnavailable, "PUSH_UNAVAILABLE", "Notificações do dispositivo ainda não estão disponíveis.")
	}
	return &messages.PushConfigResponseDTO{PublicKey: key}, nil
}

func (s *NotificationService) UpsertPushSubscription(
	ctx context.Context,
	rawKey string,
	request *messages.UpsertPushSubscriptionRequestDTO,
) (*messages.PushSubscriptionResponseDTO, error) {
	key, err := parseIdempotencyKey(rawKey)
	if err != nil {
		return nil, err
	}
	if request == nil || strings.TrimSpace(request.Endpoint) == "" || strings.TrimSpace(request.P256DH) == "" || strings.TrimSpace(request.Auth) == "" {
		return nil, mediaMomentError(http.StatusBadRequest, "INVALID_REQUEST", "endpoint, p256dh e auth são obrigatórios.")
	}
	parsed, err := url.ParseRequestURI(request.Endpoint)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || len(request.Endpoint) > 2048 || len(request.P256DH) > 512 || len(request.Auth) > 512 {
		return nil, mediaMomentError(http.StatusBadRequest, "INVALID_REQUEST", "Inscrição de dispositivo inválida.")
	}
	actor, err := requireDefaultActor(ctx, s.users, false)
	if err != nil {
		return nil, err
	}
	operation := "notifications.push_subscription.upsert"
	fingerprint := intentHash(operation, request)
	now := utcNow(s.now)
	var response *messages.PushSubscriptionResponseDTO
	err = s.WithTransaction(ctx, func(tx context.Context) error {
		prior, priorErr := findNotificationOperation(tx, s.notifications, actor.ID, key, operation, fingerprint)
		if priorErr != nil {
			return priorErr
		}
		if _, authErr := requireDefaultActor(tx, s.users, true); authErr != nil {
			return authErr
		}
		if prior != nil {
			response = &messages.PushSubscriptionResponseDTO{ID: valueOrEmpty(prior.ResultRef), State: "active", CreatedAt: now, UpdatedAt: now}
			return nil
		}
		subscription, upsertErr := s.notifications.UpsertPushSubscription(tx, &notificationEntities.PushSubscription{ID: uuid.NewString(), UserID: actor.ID, Endpoint: request.Endpoint, P256DH: request.P256DH, Auth: request.Auth, State: "active", CreatedAt: now, UpdatedAt: now})
		if upsertErr != nil {
			return appErrors.InternalError
		}
		completedAt := now
		if createErr := createNotificationOperation(tx, s.notifications, &notificationEntities.Operation{ID: uuid.NewString(), ActorUserID: actor.ID, IdempotencyKey: key, Operation: operation, IntentHash: fingerprint, State: "completed", ResultRef: &subscription.ID, ResponseSnapshot: []byte(`{}`), HTTPStatus: http.StatusOK, CreatedAt: now, CompletedAt: &completedAt}); createErr != nil {
			return createErr
		}
		response = &messages.PushSubscriptionResponseDTO{ID: subscription.ID, State: subscription.State, CreatedAt: subscription.CreatedAt.UTC(), UpdatedAt: subscription.UpdatedAt.UTC()}
		return nil
	})
	if errors.Is(err, errIdempotencyRace) {
		return s.UpsertPushSubscription(ctx, rawKey, request)
	}
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (s *NotificationService) DeactivatePushSubscription(ctx context.Context, rawKey string, request *messages.DeactivatePushSubscriptionRequestDTO) error {
	key, err := parseIdempotencyKey(rawKey)
	if err != nil {
		return err
	}
	if request == nil || strings.TrimSpace(request.Endpoint) == "" {
		return mediaMomentError(http.StatusBadRequest, "INVALID_REQUEST", "endpoint é obrigatório.")
	}
	parsed, err := url.ParseRequestURI(request.Endpoint)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || len(request.Endpoint) > 2048 {
		return mediaMomentError(http.StatusBadRequest, "INVALID_REQUEST", "Inscrição de dispositivo inválida.")
	}
	actor, err := requireDefaultActor(ctx, s.users, false)
	if err != nil {
		return err
	}
	operation := "notifications.push_subscription.deactivate"
	fingerprint := intentHash(operation, request)
	now := utcNow(s.now)
	err = s.WithTransaction(ctx, func(tx context.Context) error {
		prior, priorErr := findNotificationOperation(tx, s.notifications, actor.ID, key, operation, fingerprint)
		if priorErr != nil {
			return priorErr
		}
		if _, authErr := requireDefaultActor(tx, s.users, true); authErr != nil {
			return authErr
		}
		if prior != nil {
			return nil
		}
		if deactivateErr := s.notifications.DeactivatePushSubscription(tx, actor.ID, request.Endpoint, now); deactivateErr != nil {
			return appErrors.InternalError
		}
		completedAt := now
		return createNotificationOperation(tx, s.notifications, &notificationEntities.Operation{ID: uuid.NewString(), ActorUserID: actor.ID, IdempotencyKey: key, Operation: operation, IntentHash: fingerprint, State: "completed", ResponseSnapshot: []byte(`{}`), HTTPStatus: http.StatusNoContent, CreatedAt: now, CompletedAt: &completedAt})
	})
	if errors.Is(err, errIdempotencyRace) {
		return s.DeactivatePushSubscription(ctx, rawKey, request)
	}
	return err
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func (s *NotificationService) CreateQueueCalled(
	ctx context.Context,
	request *messages.QueueCalledNotificationRequestDTO,
) (*messages.QueueCalledNotificationResponseDTO, error) {
	if request == nil || strings.TrimSpace(request.QueueID) == "" || strings.TrimSpace(request.EntryID) == "" || uint64(request.ParticipantUserID) == 0 || request.CalledAt.IsZero() {
		return nil, mediaMomentError(http.StatusBadRequest, "INVALID_REQUEST", "queueId, entryId, participantUserId e calledAt são obrigatórios.")
	}
	now := utcNow(s.now)
	calledAt := request.CalledAt.UTC()
	sourceID := strings.TrimSpace(request.QueueID) + ":" + strings.TrimSpace(request.EntryID)
	notification := &notificationEntities.Notification{
		ID: uuid.NewString(), UserID: uint64(request.ParticipantUserID), Category: "queue_call", State: notificationEntities.StateUnread,
		Title: "É sua vez na fila", Body: "Dirija-se à fila agora.", SourceType: "pastoral_queue_call", SourceID: &sourceID, CreatedAt: calledAt,
	}
	var created bool
	err := s.WithTransaction(ctx, func(tx context.Context) error {
		createdNow, createErr := s.notifications.CreateQueueCall(tx, notification, now)
		if createErr != nil {
			return appErrors.InternalError
		}
		created = createdNow
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &messages.QueueCalledNotificationResponseDTO{NotificationID: notification.ID, Created: created}, nil
}
