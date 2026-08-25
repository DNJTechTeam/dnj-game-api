package services

import (
	"errors"
	"net/http"
	"testing"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	notificationEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/notification/entities"
	userEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func newNotificationServiceWithMocks(t *testing.T) (
	*NotificationService,
	*mocks.MockNotificationRepository,
	*mocks.MockUserRepositoryInterface,
) {
	t.Helper()
	TestSuite.DefaultSetup(t)
	notifications := mocks.NewMockNotificationRepository(t)
	users := mocks.NewMockUserRepositoryInterface(t)
	service := NewNotificationService(TestSuite.BaseService, notifications, users).(*NotificationService)
	return service, notifications, users
}

// TestNotificationService_RepositoryFailures exercises the internal-error branches that a
// real Postgres round-trip cannot reliably force: generic repository failures on every
// notification write and read path.
func TestNotificationService_RepositoryFailures(t *testing.T) {
	ctx := ctxWithUser(42)
	key := "22222222-2222-4222-8222-222222222222"

	t.Run("UpdatePreferences: rejects a nil request", func(t *testing.T) {
		service, _, _ := newNotificationServiceWithMocks(t)
		_, err := service.UpdatePreferences(ctx, key, nil)
		apiServiceError(t, err, http.StatusBadRequest, "INVALID_REQUEST")
	})

	t.Run("UpdatePreferences: rejects an invalid Idempotency-Key", func(t *testing.T) {
		service, _, _ := newNotificationServiceWithMocks(t)
		_, err := service.UpdatePreferences(ctx, "not-a-uuid", &messages.UpdateNotificationPreferencesRequestDTO{})
		apiServiceError(t, err, http.StatusBadRequest, "INVALID_REQUEST")
	})

	t.Run("UpdatePreferences: rejects a non-DEFAULT actor", func(t *testing.T) {
		service, _, users := newNotificationServiceWithMocks(t)
		mockAdminActor(users, 42)
		_, err := service.UpdatePreferences(ctx, key, &messages.UpdateNotificationPreferencesRequestDTO{})
		apiServiceError(t, err, http.StatusForbidden, "FORBIDDEN")
	})

	t.Run("UpdatePreferences: a generic re-validation failure inside the transaction is redacted", func(t *testing.T) {
		service, notifications, users := newNotificationServiceWithMocks(t)
		defaultActor := &userEntities.User{ID: 42, Role: userEntities.RoleDefault, OnboardingComplete: true}
		users.On("FindByID", mock.Anything, uint64(42)).Return(defaultActor, nil).Once()
		users.On("FindByIDForUpdate", mock.Anything, uint64(42)).Return(nil, errors.New("db down")).Once()
		notifications.On("FindOperation", mock.Anything, uint64(42), key).Return(nil, appErrors.ErrNotFound).Once()
		_, err := service.UpdatePreferences(ctx, key, &messages.UpdateNotificationPreferencesRequestDTO{})
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("UpdatePreferences: replaying a cached operation with a broken preferences lookup is redacted", func(t *testing.T) {
		service, notifications, users := newNotificationServiceWithMocks(t)
		mockDefaultActor(users, 42)
		request := &messages.UpdateNotificationPreferencesRequestDTO{}
		operation := "notifications.preferences.update"
		fingerprint := intentHash(operation, request)
		notifications.On("FindOperation", mock.Anything, uint64(42), key).
			Return(&notificationEntities.Operation{Operation: operation, IntentHash: fingerprint}, nil).Once()
		notifications.On("FindPreferences", mock.Anything, uint64(42)).Return(nil, errors.New("db down")).Once()
		_, err := service.UpdatePreferences(ctx, key, request)
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("UpdatePreferences: a conflicting write transparently retries", func(t *testing.T) {
		service, notifications, users := newNotificationServiceWithMocks(t)
		mockDefaultActor(users, 42)
		request := &messages.UpdateNotificationPreferencesRequestDTO{}
		operation := "notifications.preferences.update"
		fingerprint := intentHash(operation, request)
		notifications.On("FindOperation", mock.Anything, uint64(42), key).Return(nil, appErrors.ErrNotFound).Once()
		notifications.On("FindPreferences", mock.Anything, uint64(42)).Return(nil, appErrors.ErrNotFound).Once()
		notifications.On("UpsertPreferences", mock.Anything, mock.Anything).
			Return(&notificationEntities.Preferences{UserID: 42, PointsEnabled: true, AnnouncementEnabled: true}, nil).Once()
		notifications.On("CreateOperation", mock.Anything, mock.Anything).Return(appErrors.ErrConflict).Once()
		notifications.On("FindOperation", mock.Anything, uint64(42), key).
			Return(&notificationEntities.Operation{Operation: operation, IntentHash: fingerprint}, nil).Once()
		notifications.On("FindPreferences", mock.Anything, uint64(42)).
			Return(&notificationEntities.Preferences{UserID: 42, PointsEnabled: true, AnnouncementEnabled: true}, nil).Once()
		result, err := service.UpdatePreferences(ctx, key, request)
		assert.NoError(t, err)
		assert.True(t, result.PointsEnabled)
	})

	t.Run("GetPreferences: a generic lookup failure is redacted", func(t *testing.T) {
		service, notifications, users := newNotificationServiceWithMocks(t)
		mockDefaultActor(users, 42)
		notifications.On("FindPreferences", mock.Anything, uint64(42)).Return(nil, errors.New("db down")).Once()
		_, err := service.GetPreferences(ctx)
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("UpdatePreferences: a generic idempotency lookup failure is redacted", func(t *testing.T) {
		service, notifications, users := newNotificationServiceWithMocks(t)
		mockDefaultActor(users, 42)
		notifications.On("FindOperation", mock.Anything, uint64(42), key).Return(nil, errors.New("db down")).Once()
		_, err := service.UpdatePreferences(ctx, key, &messages.UpdateNotificationPreferencesRequestDTO{})
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("UpdatePreferences: a generic preferences lookup failure is redacted", func(t *testing.T) {
		service, notifications, users := newNotificationServiceWithMocks(t)
		mockDefaultActor(users, 42)
		notifications.On("FindOperation", mock.Anything, uint64(42), key).Return(nil, appErrors.ErrNotFound).Once()
		notifications.On("FindPreferences", mock.Anything, uint64(42)).Return(nil, errors.New("db down")).Once()
		_, err := service.UpdatePreferences(ctx, key, &messages.UpdateNotificationPreferencesRequestDTO{})
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("UpdatePreferences: a generic upsert failure is redacted", func(t *testing.T) {
		service, notifications, users := newNotificationServiceWithMocks(t)
		mockDefaultActor(users, 42)
		notifications.On("FindOperation", mock.Anything, uint64(42), key).Return(nil, appErrors.ErrNotFound).Once()
		notifications.On("FindPreferences", mock.Anything, uint64(42)).Return(nil, appErrors.ErrNotFound).Once()
		notifications.On("UpsertPreferences", mock.Anything, mock.Anything).Return(nil, errors.New("db down")).Once()
		_, err := service.UpdatePreferences(ctx, key, &messages.UpdateNotificationPreferencesRequestDTO{})
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("UpdatePreferences: a generic operation-persistence failure is redacted", func(t *testing.T) {
		service, notifications, users := newNotificationServiceWithMocks(t)
		mockDefaultActor(users, 42)
		notifications.On("FindOperation", mock.Anything, uint64(42), key).Return(nil, appErrors.ErrNotFound).Once()
		notifications.On("FindPreferences", mock.Anything, uint64(42)).Return(nil, appErrors.ErrNotFound).Once()
		notifications.On("UpsertPreferences", mock.Anything, mock.Anything).
			Return(&notificationEntities.Preferences{UserID: 42, PointsEnabled: true, AnnouncementEnabled: true}, nil).Once()
		notifications.On("CreateOperation", mock.Anything, mock.Anything).Return(errors.New("db down")).Once()
		_, err := service.UpdatePreferences(ctx, key, &messages.UpdateNotificationPreferencesRequestDTO{})
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("List: rejects a non-DEFAULT actor", func(t *testing.T) {
		service, _, users := newNotificationServiceWithMocks(t)
		mockAdminActor(users, 42)
		_, err := service.List(ctx, &messages.ListNotificationsFilterDTO{})
		apiServiceError(t, err, http.StatusForbidden, "FORBIDDEN")
	})

	t.Run("List: defaults a nil filter to the first page", func(t *testing.T) {
		service, notifications, users := newNotificationServiceWithMocks(t)
		mockDefaultActor(users, 42)
		notifications.On("List", mock.Anything, uint64(42), uint64(0)).
			Return(&messages.PaginatedResponse[notificationEntities.Notification]{}, nil).Once()
		notifications.On("CountUnread", mock.Anything, uint64(42)).Return(uint64(0), nil).Once()
		_, err := service.List(ctx, nil)
		assert.NoError(t, err)
	})

	t.Run("List: a generic notifications lookup failure is redacted", func(t *testing.T) {
		service, notifications, users := newNotificationServiceWithMocks(t)
		mockDefaultActor(users, 42)
		notifications.On("List", mock.Anything, uint64(42), uint64(0)).Return(nil, errors.New("db down")).Once()
		_, err := service.List(ctx, &messages.ListNotificationsFilterDTO{})
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("List: a generic unread-count failure is redacted", func(t *testing.T) {
		service, notifications, users := newNotificationServiceWithMocks(t)
		mockDefaultActor(users, 42)
		notifications.On("List", mock.Anything, uint64(42), uint64(0)).
			Return(&messages.PaginatedResponse[notificationEntities.Notification]{}, nil).Once()
		notifications.On("CountUnread", mock.Anything, uint64(42)).Return(uint64(0), errors.New("db down")).Once()
		_, err := service.List(ctx, &messages.ListNotificationsFilterDTO{})
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("MarkRead: rejects an invalid Idempotency-Key", func(t *testing.T) {
		service, _, _ := newNotificationServiceWithMocks(t)
		_, err := service.MarkRead(ctx, "notification-1", "not-a-uuid")
		apiServiceError(t, err, http.StatusBadRequest, "INVALID_REQUEST")
	})

	t.Run("MarkRead: rejects a non-DEFAULT actor", func(t *testing.T) {
		service, _, users := newNotificationServiceWithMocks(t)
		mockAdminActor(users, 42)
		_, err := service.MarkRead(ctx, "notification-1", key)
		apiServiceError(t, err, http.StatusForbidden, "FORBIDDEN")
	})

	t.Run("MarkRead: a generic re-validation failure inside the transaction is redacted", func(t *testing.T) {
		service, notifications, users := newNotificationServiceWithMocks(t)
		defaultActor := &userEntities.User{ID: 42, Role: userEntities.RoleDefault, OnboardingComplete: true}
		users.On("FindByID", mock.Anything, uint64(42)).Return(defaultActor, nil).Once()
		users.On("FindByIDForUpdate", mock.Anything, uint64(42)).Return(nil, errors.New("db down")).Once()
		notifications.On("FindOperation", mock.Anything, uint64(42), key).Return(nil, appErrors.ErrNotFound).Once()
		_, err := service.MarkRead(ctx, "notification-1", key)
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("MarkRead: a conflicting write transparently retries", func(t *testing.T) {
		service, notifications, users := newNotificationServiceWithMocks(t)
		mockDefaultActor(users, 42)
		operation := "notifications.read"
		fingerprint := intentHash(operation, struct {
			NotificationID string `json:"notificationId"`
		}{NotificationID: "notification-1"})
		notifications.On("FindOperation", mock.Anything, uint64(42), key).Return(nil, appErrors.ErrNotFound).Once()
		notifications.On("FindByIDAndUser", mock.Anything, "notification-1", uint64(42)).
			Return(&notificationEntities.Notification{ID: "notification-1", State: notificationEntities.StateUnread}, nil).Once()
		notifications.On("MarkRead", mock.Anything, "notification-1", uint64(42), mock.Anything).
			Return(&notificationEntities.Notification{ID: "notification-1", State: notificationEntities.StateRead}, nil).Once()
		notifications.On("CreateOperation", mock.Anything, mock.Anything).Return(appErrors.ErrConflict).Once()
		notifications.On("FindOperation", mock.Anything, uint64(42), key).
			Return(&notificationEntities.Operation{Operation: operation, IntentHash: fingerprint}, nil).Once()
		notifications.On("FindByIDAndUser", mock.Anything, "notification-1", uint64(42)).
			Return(&notificationEntities.Notification{ID: "notification-1", State: notificationEntities.StateRead}, nil).Once()
		result, err := service.MarkRead(ctx, "notification-1", key)
		assert.NoError(t, err)
		assert.Equal(t, "read", result.State)
	})

	t.Run("MarkRead: a generic idempotency lookup failure is redacted", func(t *testing.T) {
		service, notifications, users := newNotificationServiceWithMocks(t)
		mockDefaultActor(users, 42)
		notifications.On("FindOperation", mock.Anything, uint64(42), key).Return(nil, errors.New("db down")).Once()
		_, err := service.MarkRead(ctx, "notification-1", key)
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("MarkRead: a generic notification lookup failure is redacted", func(t *testing.T) {
		service, notifications, users := newNotificationServiceWithMocks(t)
		mockDefaultActor(users, 42)
		notifications.On("FindOperation", mock.Anything, uint64(42), key).Return(nil, appErrors.ErrNotFound).Once()
		notifications.On("FindByIDAndUser", mock.Anything, "notification-1", uint64(42)).
			Return(nil, errors.New("db down")).Once()
		_, err := service.MarkRead(ctx, "notification-1", key)
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("MarkRead: replays a cached result without writing again", func(t *testing.T) {
		service, notifications, users := newNotificationServiceWithMocks(t)
		mockDefaultActor(users, 42)
		resourceRef := "notification-1"
		operation := "notifications.read"
		fingerprint := intentHash(operation, struct {
			NotificationID string `json:"notificationId"`
		}{NotificationID: "notification-1"})
		notifications.On("FindOperation", mock.Anything, uint64(42), key).
			Return(&notificationEntities.Operation{Operation: operation, IntentHash: fingerprint, ResourceRef: &resourceRef}, nil).Once()
		notifications.On("FindByIDAndUser", mock.Anything, "notification-1", uint64(42)).
			Return(&notificationEntities.Notification{ID: "notification-1", State: notificationEntities.StateRead}, nil).Once()
		result, err := service.MarkRead(ctx, "notification-1", key)
		assert.NoError(t, err)
		assert.Equal(t, "read", result.State)
	})

	t.Run("MarkRead: a generic write failure is redacted", func(t *testing.T) {
		service, notifications, users := newNotificationServiceWithMocks(t)
		mockDefaultActor(users, 42)
		notifications.On("FindOperation", mock.Anything, uint64(42), key).Return(nil, appErrors.ErrNotFound).Once()
		notifications.On("FindByIDAndUser", mock.Anything, "notification-1", uint64(42)).
			Return(&notificationEntities.Notification{ID: "notification-1", State: notificationEntities.StateUnread}, nil).Once()
		notifications.On("MarkRead", mock.Anything, "notification-1", uint64(42), mock.Anything).
			Return(nil, errors.New("db down")).Once()
		_, err := service.MarkRead(ctx, "notification-1", key)
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("MarkRead: a generic operation-persistence failure is redacted", func(t *testing.T) {
		service, notifications, users := newNotificationServiceWithMocks(t)
		mockDefaultActor(users, 42)
		notifications.On("FindOperation", mock.Anything, uint64(42), key).Return(nil, appErrors.ErrNotFound).Once()
		notifications.On("FindByIDAndUser", mock.Anything, "notification-1", uint64(42)).
			Return(&notificationEntities.Notification{ID: "notification-1", State: notificationEntities.StateUnread}, nil).Once()
		notifications.On("MarkRead", mock.Anything, "notification-1", uint64(42), mock.Anything).
			Return(&notificationEntities.Notification{ID: "notification-1", State: notificationEntities.StateRead}, nil).Once()
		notifications.On("CreateOperation", mock.Anything, mock.Anything).Return(errors.New("db down")).Once()
		_, err := service.MarkRead(ctx, "notification-1", key)
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("AdminSend: rejects an invalid Idempotency-Key", func(t *testing.T) {
		service, _, _ := newNotificationServiceWithMocks(t)
		_, err := service.AdminSend(ctx, "not-a-uuid", &messages.AdminSendNotificationRequestDTO{Title: "T", Body: "B"})
		apiServiceError(t, err, http.StatusBadRequest, "INVALID_REQUEST")
	})

	t.Run("AdminSend: a generic re-validation failure inside the transaction is redacted", func(t *testing.T) {
		service, notifications, users := newNotificationServiceWithMocks(t)
		adminActor := &userEntities.User{ID: 42, Role: userEntities.RoleAdmin, OnboardingComplete: true}
		users.On("FindByID", mock.Anything, uint64(42)).Return(adminActor, nil).Once()
		users.On("FindByID", mock.Anything, uint64(42)).Return(nil, errors.New("db down")).Once()
		notifications.On("FindOperation", mock.Anything, uint64(42), key).Return(nil, appErrors.ErrNotFound).Once()
		_, err := service.AdminSend(ctx, key, &messages.AdminSendNotificationRequestDTO{Title: "T", Body: "B"})
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("AdminSend: a conflicting write transparently retries", func(t *testing.T) {
		service, notifications, users := newNotificationServiceWithMocks(t)
		mockAdminActor(users, 42)
		request := &messages.AdminSendNotificationRequestDTO{Title: "T", Body: "B"}
		operation := "admin.notifications.send"
		fingerprint := intentHash(operation, request)
		notifications.On("FindOperation", mock.Anything, uint64(42), key).Return(nil, appErrors.ErrNotFound).Once()
		notifications.On("ResolveAnnouncementRecipients", mock.Anything, mock.Anything).Return([]uint64{7}, nil).Once()
		notifications.On("CreateBroadcast", mock.Anything, mock.Anything).Return(nil).Once()
		notifications.On("CreateOperation", mock.Anything, mock.Anything).Return(appErrors.ErrConflict).Once()
		count := 1
		notifications.On("FindOperation", mock.Anything, uint64(42), key).
			Return(&notificationEntities.Operation{Operation: operation, IntentHash: fingerprint, ResultCount: &count}, nil).Once()
		result, err := service.AdminSend(ctx, key, request)
		assert.NoError(t, err)
		assert.Equal(t, messages.Uint64StringFromUint64(1), result.RecipientCount)
	})

	t.Run("AdminSend: a generic idempotency lookup failure is redacted", func(t *testing.T) {
		service, notifications, users := newNotificationServiceWithMocks(t)
		mockAdminActor(users, 42)
		notifications.On("FindOperation", mock.Anything, uint64(42), key).Return(nil, errors.New("db down")).Once()
		_, err := service.AdminSend(ctx, key, &messages.AdminSendNotificationRequestDTO{Title: "T", Body: "B"})
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("AdminSend: replays a cached recipient count without a second write", func(t *testing.T) {
		service, notifications, users := newNotificationServiceWithMocks(t)
		mockAdminActor(users, 42)
		request := &messages.AdminSendNotificationRequestDTO{Title: "T", Body: "B"}
		operation := "admin.notifications.send"
		fingerprint := intentHash(operation, request)
		count := 3
		notifications.On("FindOperation", mock.Anything, uint64(42), key).
			Return(&notificationEntities.Operation{Operation: operation, IntentHash: fingerprint, ResultCount: &count}, nil).Once()
		result, err := service.AdminSend(ctx, key, request)
		assert.NoError(t, err)
		assert.Equal(t, messages.Uint64StringFromUint64(3), result.RecipientCount)
	})

	t.Run("AdminSend: replays a cached operation without a cached count as zero", func(t *testing.T) {
		service, notifications, users := newNotificationServiceWithMocks(t)
		mockAdminActor(users, 42)
		request := &messages.AdminSendNotificationRequestDTO{Title: "T", Body: "B"}
		operation := "admin.notifications.send"
		fingerprint := intentHash(operation, request)
		notifications.On("FindOperation", mock.Anything, uint64(42), key).
			Return(&notificationEntities.Operation{Operation: operation, IntentHash: fingerprint}, nil).Once()
		result, err := service.AdminSend(ctx, key, request)
		assert.NoError(t, err)
		assert.Equal(t, messages.Uint64StringFromUint64(0), result.RecipientCount)
	})

	t.Run("AdminSend: a generic recipient-resolution failure is redacted", func(t *testing.T) {
		service, notifications, users := newNotificationServiceWithMocks(t)
		mockAdminActor(users, 42)
		notifications.On("FindOperation", mock.Anything, uint64(42), key).Return(nil, appErrors.ErrNotFound).Once()
		notifications.On("ResolveAnnouncementRecipients", mock.Anything, mock.Anything).
			Return(nil, errors.New("db down")).Once()
		_, err := service.AdminSend(ctx, key, &messages.AdminSendNotificationRequestDTO{Title: "T", Body: "B"})
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("AdminSend: a generic broadcast-write failure is redacted", func(t *testing.T) {
		service, notifications, users := newNotificationServiceWithMocks(t)
		mockAdminActor(users, 42)
		notifications.On("FindOperation", mock.Anything, uint64(42), key).Return(nil, appErrors.ErrNotFound).Once()
		notifications.On("ResolveAnnouncementRecipients", mock.Anything, mock.Anything).
			Return([]uint64{7}, nil).Once()
		notifications.On("CreateBroadcast", mock.Anything, mock.Anything).Return(errors.New("db down")).Once()
		_, err := service.AdminSend(ctx, key, &messages.AdminSendNotificationRequestDTO{Title: "T", Body: "B"})
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("AdminSend: a generic operation-persistence failure is redacted", func(t *testing.T) {
		service, notifications, users := newNotificationServiceWithMocks(t)
		mockAdminActor(users, 42)
		notifications.On("FindOperation", mock.Anything, uint64(42), key).Return(nil, appErrors.ErrNotFound).Once()
		notifications.On("ResolveAnnouncementRecipients", mock.Anything, mock.Anything).
			Return([]uint64{7}, nil).Once()
		notifications.On("CreateBroadcast", mock.Anything, mock.Anything).Return(nil).Once()
		notifications.On("CreateOperation", mock.Anything, mock.Anything).Return(errors.New("db down")).Once()
		_, err := service.AdminSend(ctx, key, &messages.AdminSendNotificationRequestDTO{Title: "T", Body: "B"})
		assert.ErrorIs(t, err, appErrors.InternalError)
	})
}
