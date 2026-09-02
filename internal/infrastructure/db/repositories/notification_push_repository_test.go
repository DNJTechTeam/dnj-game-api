package repositories

import (
	"context"
	"testing"
	"time"

	notificationEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/notification/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotifications_PushSubscriptionAndOutboxLifecycle(t *testing.T) {
	ctx := context.Background()
	TestSuite.TruncateTable(t, &models.NotificationDelivery{})
	TestSuite.TruncateTable(t, &models.PushSubscription{})
	TestSuite.TruncateTable(t, &models.Notification{})
	TestSuite.TruncateTable(t, &models.NotificationPreference{})
	TestSuite.TruncateTable(t, &models.User{})

	repo := NewNotificationRepository(TestSuite.DbConn)
	user := seedUser(t, ctx, "notification-push@example.com")
	now := time.Now().UTC()

	first, err := repo.UpsertPushSubscription(ctx, &notificationEntities.PushSubscription{
		ID: uuid.NewString(), UserID: user.ID, Endpoint: "https://push.example/subscription", P256DH: "key-1", Auth: "auth-1", State: "active", CreatedAt: now, UpdatedAt: now,
	})
	require.NoError(t, err)
	require.Equal(t, "key-1", first.P256DH)

	updated, err := repo.UpsertPushSubscription(ctx, &notificationEntities.PushSubscription{
		ID: uuid.NewString(), UserID: user.ID, Endpoint: first.Endpoint, P256DH: "key-2", Auth: "auth-2", State: "active", CreatedAt: now, UpdatedAt: now.Add(time.Minute),
	})
	require.NoError(t, err)
	require.Equal(t, first.ID, updated.ID)
	require.Equal(t, "key-2", updated.P256DH)

	activityID := uuid.NewString()
	notification := &notificationEntities.Notification{ID: uuid.NewString(), UserID: user.ID, Category: notificationEntities.CategoryChallenge, State: notificationEntities.StateUnread, Title: "Novo desafio", Body: "Participe agora", SourceType: "activity", SourceID: &activityID, CreatedAt: now}
	require.NoError(t, repo.CreateBroadcast(ctx, []*notificationEntities.Notification{notification}))

	var delivery models.NotificationDelivery
	require.NoError(t, TestSuite.DbConn.Where("notification_id = ?", notification.ID).Take(&delivery).Error)
	assert.Equal(t, "pending", delivery.State)
	assert.Equal(t, updated.ID, delivery.SubscriptionID)

	require.NoError(t, repo.DeactivatePushSubscription(ctx, user.ID, updated.Endpoint, now.Add(time.Minute)))
	var inactive models.PushSubscription
	require.NoError(t, TestSuite.DbConn.Where("id = ?", updated.ID).Take(&inactive).Error)
	assert.Equal(t, "inactive", inactive.State)

	active, err := repo.UpsertPushSubscription(ctx, &notificationEntities.PushSubscription{
		ID: uuid.NewString(), UserID: user.ID, Endpoint: updated.Endpoint, P256DH: "key-3", Auth: "auth-3", State: "active", CreatedAt: now, UpdatedAt: now.Add(2 * time.Minute),
	})
	require.NoError(t, err)
	queueID := uuid.NewString()
	queueCall := &notificationEntities.Notification{ID: uuid.NewString(), UserID: user.ID, Category: "queue_call", State: notificationEntities.StateUnread, Title: "É sua vez", Body: "Dirija-se à fila", SourceType: "pastoral_queue_call", SourceID: &queueID, CreatedAt: now}
	created, err := repo.CreateQueueCall(ctx, queueCall, now)
	require.NoError(t, err)
	assert.True(t, created)

	duplicate := &notificationEntities.Notification{ID: uuid.NewString(), UserID: user.ID, Category: "queue_call", State: notificationEntities.StateUnread, Title: "É sua vez", Body: "Dirija-se à fila", SourceType: "pastoral_queue_call", SourceID: &queueID, CreatedAt: now}
	created, err = repo.CreateQueueCall(ctx, duplicate, now)
	require.NoError(t, err)
	assert.False(t, created)
	assert.Equal(t, queueCall.ID, duplicate.ID)

	var queueDelivery models.NotificationDelivery
	require.NoError(t, TestSuite.DbConn.Where("notification_id = ?", queueCall.ID).Take(&queueDelivery).Error)
	assert.Equal(t, active.ID, queueDelivery.SubscriptionID)
}

func TestNotifications_DerivedNotificationHonorsPreferences(t *testing.T) {
	ctx := context.Background()
	TestSuite.TruncateTable(t, &models.NotificationDelivery{})
	TestSuite.TruncateTable(t, &models.PushSubscription{})
	TestSuite.TruncateTable(t, &models.Notification{})
	TestSuite.TruncateTable(t, &models.NotificationPreference{})
	TestSuite.TruncateTable(t, &models.User{})

	user := seedUser(t, ctx, "notification-preferences@example.com")
	now := time.Now().UTC()
	require.NoError(t, TestSuite.DbConn.Create(&models.NotificationPreference{UserID: user.ID, PointsEnabled: false, AnnouncementEnabled: false, UpdatedAt: now}).Error)
	require.NoError(t, TestSuite.DbConn.Create(&models.PushSubscription{ID: uuid.NewString(), UserID: user.ID, Endpoint: "https://push.example/preference", P256DH: "key", Auth: "auth", State: "active", CreatedAt: now, UpdatedAt: now}).Error)

	require.NoError(t, writeDerivedNotification(ctx, TestSuite.DbConn, user.ID, "points", "Pontos", "Você ganhou pontos", "moment", uuid.NewString(), now))
	var count int64
	require.NoError(t, TestSuite.DbConn.Model(&models.Notification{}).Where("user_id = ?", user.ID).Count(&count).Error)
	assert.Zero(t, count)

	require.NoError(t, writeDerivedNotification(ctx, TestSuite.DbConn, user.ID, "moment_moderation", "Foto removida", "Sua foto foi removida", "moment", uuid.NewString(), now))
	require.NoError(t, TestSuite.DbConn.Model(&models.Notification{}).Where("user_id = ?", user.ID).Count(&count).Error)
	assert.Equal(t, int64(1), count)
	require.NoError(t, TestSuite.DbConn.Model(&models.NotificationDelivery{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}
