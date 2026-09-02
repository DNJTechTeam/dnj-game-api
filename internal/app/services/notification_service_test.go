package services

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/dnjtechteam/dnj-game-api/internal/app/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	userEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/repositories"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var notificationServiceNow = time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)

func setupNotificationServiceTest(t *testing.T) interfaces.NotificationServiceInterface {
	t.Helper()
	TestSuite.DefaultSetup(t)
	TestSuite.TruncateTable(t, &models.NotificationDelivery{})
	TestSuite.TruncateTable(t, &models.PushSubscription{})
	TestSuite.TruncateTable(t, &models.Notification{})
	TestSuite.TruncateTable(t, &models.NotificationPreference{})
	TestSuite.TruncateTable(t, &models.IdempotencyOperation{})
	TestSuite.TruncateTable(t, &models.User{})
	t.Cleanup(func() {
		TestSuite.TruncateTable(t, &models.NotificationDelivery{})
		TestSuite.TruncateTable(t, &models.PushSubscription{})
		TestSuite.TruncateTable(t, &models.Notification{})
		TestSuite.TruncateTable(t, &models.NotificationPreference{})
		TestSuite.TruncateTable(t, &models.IdempotencyOperation{})
		TestSuite.TruncateTable(t, &models.User{})
	})
	repository := repositories.NewNotificationRepository(TestSuite.DbConn)
	service := NewNotificationService(TestSuite.BaseService, repository, TestSuite.UserRepository).(*NotificationService)
	service.now = func() time.Time { return notificationServiceNow }
	return service
}

func seedNotificationUser(
	t *testing.T,
	email string,
	role userEntities.UserRole,
	onboarding bool,
) (*userEntities.User, context.Context) {
	t.Helper()
	user, err := TestSuite.UserRepository.Create(TestSuite.Ctx, &userEntities.User{
		Email: email, Name: "Notificação", Role: role, OnboardingComplete: onboarding,
	})
	require.NoError(t, err)
	return user, TestSuite.ContextWithUser(user.ID)
}

func seedNotification(t *testing.T, userID uint64, category, state string, createdAt time.Time) *models.Notification {
	t.Helper()
	row := &models.Notification{
		ID: uuid.NewString(), UserID: userID, Category: category, State: state,
		Title: "Título", Body: "Corpo", SourceType: "test", CreatedAt: createdAt,
	}
	require.NoError(t, TestSuite.DbConn.Create(row).Error)
	return row
}

func TestNotificationService_Preferences(t *testing.T) {
	t.Run("returns enabled-by-default preferences with moderation always on", func(t *testing.T) {
		// given
		service := setupNotificationServiceTest(t)
		_, ctx := seedNotificationUser(t, "prefs-default@example.com", userEntities.RoleDefault, true)

		// when
		result, err := service.GetPreferences(ctx)

		// then
		require.NoError(t, err)
		assert.True(t, result.MomentModerationEnabled)
		assert.True(t, result.PointsEnabled)
		assert.True(t, result.AnnouncementEnabled)
	})

	t.Run("updates and persists points and announcement preferences", func(t *testing.T) {
		// given
		service := setupNotificationServiceTest(t)
		_, ctx := seedNotificationUser(t, "prefs-update@example.com", userEntities.RoleDefault, true)
		points := false
		announcement := false

		// when
		updated, err := service.UpdatePreferences(ctx, uuid.NewString(), &messages.UpdateNotificationPreferencesRequestDTO{
			PointsEnabled: &points, AnnouncementEnabled: &announcement,
		})
		fetched, fetchErr := service.GetPreferences(ctx)

		// then
		require.NoError(t, err)
		require.NoError(t, fetchErr)
		assert.False(t, updated.PointsEnabled)
		assert.False(t, updated.AnnouncementEnabled)
		assert.True(t, updated.MomentModerationEnabled)
		assert.False(t, fetched.PointsEnabled)
		assert.False(t, fetched.AnnouncementEnabled)
	})

	t.Run("replays update with the same Idempotency-Key without changing state twice", func(t *testing.T) {
		// given
		service := setupNotificationServiceTest(t)
		_, ctx := seedNotificationUser(t, "prefs-replay@example.com", userEntities.RoleDefault, true)
		key := uuid.NewString()
		points := false
		request := &messages.UpdateNotificationPreferencesRequestDTO{PointsEnabled: &points}

		// when
		first, err1 := service.UpdatePreferences(ctx, key, request)
		second, err2 := service.UpdatePreferences(ctx, key, request)

		// then
		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.Equal(t, first.PointsEnabled, second.PointsEnabled)
		var count int64
		require.NoError(t, TestSuite.DbConn.Model(&models.IdempotencyOperation{}).
			Where("idempotency_key = ?", key).Count(&count).Error)
		assert.Equal(t, int64(1), count)
	})

	t.Run("rejects a reused Idempotency-Key sent with a different intent", func(t *testing.T) {
		// given
		service := setupNotificationServiceTest(t)
		_, ctx := seedNotificationUser(t, "prefs-conflict@example.com", userEntities.RoleDefault, true)
		key := uuid.NewString()
		points := false
		announcement := false

		// when
		_, err1 := service.UpdatePreferences(ctx, key, &messages.UpdateNotificationPreferencesRequestDTO{PointsEnabled: &points})
		_, err2 := service.UpdatePreferences(ctx, key, &messages.UpdateNotificationPreferencesRequestDTO{AnnouncementEnabled: &announcement})

		// then
		require.NoError(t, err1)
		apiServiceError(t, err2, http.StatusConflict, "IDEMPOTENCY_KEY_REUSED")
	})

	t.Run("rejects update without a valid Idempotency-Key", func(t *testing.T) {
		// given
		service := setupNotificationServiceTest(t)
		_, ctx := seedNotificationUser(t, "prefs-badkey@example.com", userEntities.RoleDefault, true)

		// when
		_, err := service.UpdatePreferences(ctx, "not-a-uuid", &messages.UpdateNotificationPreferencesRequestDTO{})

		// then
		apiServiceError(t, err, http.StatusBadRequest, "INVALID_REQUEST")
	})

	t.Run("rejects preferences access from a non-DEFAULT role", func(t *testing.T) {
		// given
		service := setupNotificationServiceTest(t)
		_, ctx := seedNotificationUser(t, "prefs-admin@example.com", userEntities.RoleAdmin, true)

		// when
		_, err := service.GetPreferences(ctx)

		// then
		apiServiceError(t, err, http.StatusForbidden, "FORBIDDEN")
	})

	t.Run("rejects preferences access before onboarding completes", func(t *testing.T) {
		// given
		service := setupNotificationServiceTest(t)
		_, ctx := seedNotificationUser(t, "prefs-onboarding@example.com", userEntities.RoleDefault, false)

		// when
		_, err := service.GetPreferences(ctx)

		// then
		apiServiceError(t, err, http.StatusConflict, "ONBOARDING_REQUIRED")
	})
}

func TestNotificationService_List(t *testing.T) {
	t.Run("lists notifications newest first with an unread badge count", func(t *testing.T) {
		// given
		service := setupNotificationServiceTest(t)
		user, ctx := seedNotificationUser(t, "list-user@example.com", userEntities.RoleDefault, true)
		older := seedNotification(t, user.ID, "points", "unread", notificationServiceNow.Add(-time.Hour))
		newer := seedNotification(t, user.ID, "announcement", "read", notificationServiceNow)
		other, _ := seedNotificationUser(t, "list-other@example.com", userEntities.RoleDefault, true)
		seedNotification(t, other.ID, "points", "unread", notificationServiceNow)

		// when
		result, err := service.List(ctx, &messages.ListNotificationsFilterDTO{})

		// then
		require.NoError(t, err)
		require.Len(t, result.Data, 2)
		assert.Equal(t, newer.ID, result.Data[0].ID)
		assert.Equal(t, older.ID, result.Data[1].ID)
		assert.Equal(t, messages.Uint64StringFromUint64(1), result.UnreadCount)
	})

	t.Run("paginates across pages", func(t *testing.T) {
		// given
		service := setupNotificationServiceTest(t)
		user, ctx := seedNotificationUser(t, "list-paginated@example.com", userEntities.RoleDefault, true)
		for i := 0; i < 11; i++ {
			seedNotification(t, user.ID, "points", "unread", notificationServiceNow.Add(-time.Duration(i)*time.Minute))
		}

		// when
		firstPage, err := service.List(ctx, &messages.ListNotificationsFilterDTO{})
		filter := &messages.ListNotificationsFilterDTO{}
		filter.SetPage(1)
		secondPage, err2 := service.List(ctx, filter)

		// then
		require.NoError(t, err)
		require.NoError(t, err2)
		assert.Len(t, firstPage.Data, 10)
		assert.True(t, firstPage.Pagination.HasNextPage)
		assert.Len(t, secondPage.Data, 1)
		assert.False(t, secondPage.Pagination.HasNextPage)
	})
}

func TestNotificationService_MarkRead(t *testing.T) {
	t.Run("marks an unread notification as read", func(t *testing.T) {
		// given
		service := setupNotificationServiceTest(t)
		user, ctx := seedNotificationUser(t, "read-user@example.com", userEntities.RoleDefault, true)
		notification := seedNotification(t, user.ID, "points", "unread", notificationServiceNow)

		// when
		result, err := service.MarkRead(ctx, notification.ID, uuid.NewString())

		// then
		require.NoError(t, err)
		assert.Equal(t, "read", result.State)
		require.NotNil(t, result.ReadAt)
	})

	t.Run("marking an already-read notification is idempotent and never reverts to unread", func(t *testing.T) {
		// given
		service := setupNotificationServiceTest(t)
		user, ctx := seedNotificationUser(t, "read-idempotent@example.com", userEntities.RoleDefault, true)
		notification := seedNotification(t, user.ID, "points", "unread", notificationServiceNow)

		// when
		first, err1 := service.MarkRead(ctx, notification.ID, uuid.NewString())
		second, err2 := service.MarkRead(ctx, notification.ID, uuid.NewString())

		// then
		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.Equal(t, "read", first.State)
		assert.Equal(t, "read", second.State)
		assert.Equal(t, first.ReadAt, second.ReadAt)
	})

	t.Run("replays with the same Idempotency-Key without a second write", func(t *testing.T) {
		// given
		service := setupNotificationServiceTest(t)
		user, ctx := seedNotificationUser(t, "read-replay@example.com", userEntities.RoleDefault, true)
		notification := seedNotification(t, user.ID, "points", "unread", notificationServiceNow)
		key := uuid.NewString()

		// when
		first, err1 := service.MarkRead(ctx, notification.ID, key)
		second, err2 := service.MarkRead(ctx, notification.ID, key)

		// then
		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.Equal(t, first.ReadAt, second.ReadAt)
	})

	t.Run("returns a uniform not-found for another user's notification", func(t *testing.T) {
		// given
		service := setupNotificationServiceTest(t)
		owner, _ := seedNotificationUser(t, "read-owner@example.com", userEntities.RoleDefault, true)
		_, strangerCtx := seedNotificationUser(t, "read-stranger@example.com", userEntities.RoleDefault, true)
		notification := seedNotification(t, owner.ID, "points", "unread", notificationServiceNow)

		// when
		_, err := service.MarkRead(strangerCtx, notification.ID, uuid.NewString())

		// then
		apiServiceError(t, err, http.StatusNotFound, "NOT_FOUND")
	})

	t.Run("returns a uniform not-found for a nonexistent notification", func(t *testing.T) {
		// given
		service := setupNotificationServiceTest(t)
		_, ctx := seedNotificationUser(t, "read-missing@example.com", userEntities.RoleDefault, true)

		// when
		_, err := service.MarkRead(ctx, uuid.NewString(), uuid.NewString())

		// then
		apiServiceError(t, err, http.StatusNotFound, "NOT_FOUND")
	})

	t.Run("returns a uniform not-found for a malformed notification id instead of a 500", func(t *testing.T) {
		// given
		service := setupNotificationServiceTest(t)
		_, ctx := seedNotificationUser(t, "read-malformed@example.com", userEntities.RoleDefault, true)

		// when
		_, err := service.MarkRead(ctx, "not-a-uuid", uuid.NewString())

		// then
		apiServiceError(t, err, http.StatusNotFound, "NOT_FOUND")
	})
}

func TestNotificationService_AdminSend(t *testing.T) {
	t.Run("broadcasts to eligible DEFAULT users and exposes only the aggregate count", func(t *testing.T) {
		// given
		service := setupNotificationServiceTest(t)
		_, adminCtx := seedNotificationUser(t, "admin-send@example.com", userEntities.RoleAdmin, true)
		recipient, _ := seedNotificationUser(t, "recipient@example.com", userEntities.RoleDefault, true)
		optedOut, optedOutCtx := seedNotificationUser(t, "opted-out@example.com", userEntities.RoleDefault, true)
		notOnboarded, _ := seedNotificationUser(t, "not-onboarded@example.com", userEntities.RoleDefault, false)
		announcement := false
		_, err := service.UpdatePreferences(optedOutCtx, uuid.NewString(), &messages.UpdateNotificationPreferencesRequestDTO{
			AnnouncementEnabled: &announcement,
		})
		require.NoError(t, err)

		// when
		result, sendErr := service.AdminSend(adminCtx, uuid.NewString(), &messages.AdminSendNotificationRequestDTO{
			Title: "Aviso", Body: "Manutenção agendada",
		})
		recipientList, listErr := service.List(TestSuite.ContextWithUser(recipient.ID), &messages.ListNotificationsFilterDTO{})

		// then
		require.NoError(t, sendErr)
		require.NoError(t, listErr)
		assert.Equal(t, messages.Uint64StringFromUint64(1), result.RecipientCount)
		require.Len(t, recipientList.Data, 1)
		assert.Equal(t, "announcement", recipientList.Data[0].Category)
		var optedOutCount int64
		require.NoError(t, TestSuite.DbConn.Model(&models.Notification{}).
			Where("user_id = ?", optedOut.ID).Count(&optedOutCount).Error)
		assert.Zero(t, optedOutCount)
		var notOnboardedCount int64
		require.NoError(t, TestSuite.DbConn.Model(&models.Notification{}).
			Where("user_id = ?", notOnboarded.ID).Count(&notOnboardedCount).Error)
		assert.Zero(t, notOnboardedCount)
	})

	t.Run("narrows the broadcast to explicit target user ids", func(t *testing.T) {
		// given
		service := setupNotificationServiceTest(t)
		_, adminCtx := seedNotificationUser(t, "admin-explicit@example.com", userEntities.RoleAdmin, true)
		target, _ := seedNotificationUser(t, "explicit-target@example.com", userEntities.RoleDefault, true)
		_, _ = seedNotificationUser(t, "explicit-excluded@example.com", userEntities.RoleDefault, true)

		// when
		result, err := service.AdminSend(adminCtx, uuid.NewString(), &messages.AdminSendNotificationRequestDTO{
			Title: "Aviso", Body: "Somente você",
			TargetUserIds: []messages.Uint64String{messages.Uint64StringFromUint64(target.ID)},
		})

		// then
		require.NoError(t, err)
		assert.Equal(t, messages.Uint64StringFromUint64(1), result.RecipientCount)
	})

	t.Run("does not duplicate delivery on retry with the same Idempotency-Key", func(t *testing.T) {
		// given
		service := setupNotificationServiceTest(t)
		_, adminCtx := seedNotificationUser(t, "admin-retry@example.com", userEntities.RoleAdmin, true)
		recipient, _ := seedNotificationUser(t, "retry-recipient@example.com", userEntities.RoleDefault, true)
		key := uuid.NewString()
		request := &messages.AdminSendNotificationRequestDTO{Title: "Aviso", Body: "Repetido"}

		// when
		first, err1 := service.AdminSend(adminCtx, key, request)
		second, err2 := service.AdminSend(adminCtx, key, request)

		// then
		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.Equal(t, first.RecipientCount, second.RecipientCount)
		var count int64
		require.NoError(t, TestSuite.DbConn.Model(&models.Notification{}).
			Where("user_id = ?", recipient.ID).Count(&count).Error)
		assert.Equal(t, int64(1), count)
	})

	t.Run("rejects a blank title or body", func(t *testing.T) {
		// given
		service := setupNotificationServiceTest(t)
		_, adminCtx := seedNotificationUser(t, "admin-blank@example.com", userEntities.RoleAdmin, true)

		// when
		_, err := service.AdminSend(adminCtx, uuid.NewString(), &messages.AdminSendNotificationRequestDTO{Title: " ", Body: ""})

		// then
		apiServiceError(t, err, http.StatusBadRequest, "INVALID_REQUEST")
	})

	t.Run("rejects an EVENT_MANAGER actor", func(t *testing.T) {
		// given
		service := setupNotificationServiceTest(t)
		_, managerCtx := seedNotificationUser(t, "event-manager@example.com", userEntities.RoleEventManager, true)

		// when
		_, err := service.AdminSend(managerCtx, uuid.NewString(), &messages.AdminSendNotificationRequestDTO{
			Title: "Aviso", Body: "Corpo",
		})

		// then
		apiServiceError(t, err, http.StatusForbidden, "FORBIDDEN")
	})

	t.Run("rejects a DEFAULT actor", func(t *testing.T) {
		// given
		service := setupNotificationServiceTest(t)
		_, ctx := seedNotificationUser(t, "default-actor@example.com", userEntities.RoleDefault, true)

		// when
		_, err := service.AdminSend(ctx, uuid.NewString(), &messages.AdminSendNotificationRequestDTO{
			Title: "Aviso", Body: "Corpo",
		})

		// then
		apiServiceError(t, err, http.StatusForbidden, "FORBIDDEN")
	})
}
