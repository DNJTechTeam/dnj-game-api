package push

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/SherClockHolmes/webpush-go"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
	"gorm.io/gorm"
)

type Worker struct{ db *gorm.DB }

func NewWorker(db *gorm.DB) *Worker { return &Worker{db: db} }

// RunOnce dispatches the persisted outbox after its source transaction has
// committed. A provider failure updates only delivery state, never the event.
func (w *Worker) RunOnce(ctx context.Context) error {
	if os.Getenv("VAPID_PUBLIC_KEY") == "" || os.Getenv("VAPID_PRIVATE_KEY") == "" || os.Getenv("VAPID_SUBJECT") == "" {
		return nil
	}
	var deliveries []models.NotificationDelivery
	if err := w.db.WithContext(ctx).Where("state IN ? AND (next_attempt_at IS NULL OR next_attempt_at <= ?)", []string{"pending", "retrying"}, time.Now().UTC()).Order("created_at").Limit(50).Find(&deliveries).Error; err != nil {
		return err
	}
	for _, delivery := range deliveries {
		w.dispatch(ctx, &delivery)
	}
	return nil
}

func (w *Worker) dispatch(ctx context.Context, delivery *models.NotificationDelivery) {
	var subscription models.PushSubscription
	var notification models.Notification
	if w.db.WithContext(ctx).First(&subscription, "id = ? AND state = ?", delivery.SubscriptionID, "active").Error != nil || w.db.First(&notification, "id = ?", delivery.NotificationID).Error != nil {
		return
	}
	url := "/?screen=home"
	vibrate := []int(nil)
	if notification.Category == "queue_call" {
		url, vibrate = "/?screen=queue", []int{200, 100, 200}
	} else if notification.Category == "challenge" || notification.Category == "moment_challenge" || notification.Category == "special_event" {
		url = "/?screen=game"
	} else if notification.Category == "moment_moderation" {
		url = "/?screen=gallery"
	} else if notification.Category == "announcement" {
		var metadata struct {
			Urgent bool `json:"urgent"`
		}
		if json.Unmarshal(notification.Metadata, &metadata) == nil && metadata.Urgent {
			vibrate = []int{200, 100, 200}
		}
	}
	payload, _ := json.Marshal(map[string]any{"notificationId": notification.ID, "title": notification.Title, "body": notification.Body, "url": url, "tag": notification.ID, "vibrate": vibrate})
	resp, err := webpush.SendNotification(payload, &webpush.Subscription{Endpoint: subscription.Endpoint, Keys: webpush.Keys{P256dh: subscription.P256DH, Auth: subscription.Auth}}, &webpush.Options{Subscriber: os.Getenv("VAPID_SUBJECT"), VAPIDPublicKey: os.Getenv("VAPID_PUBLIC_KEY"), VAPIDPrivateKey: os.Getenv("VAPID_PRIVATE_KEY"), TTL: 60})
	now := time.Now().UTC()
	if resp != nil {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusGone || resp.StatusCode == http.StatusNotFound {
			w.db.WithContext(ctx).Model(&subscription).Updates(map[string]any{"state": "inactive", "disabled_at": now, "updated_at": now})
			w.db.WithContext(ctx).Model(delivery).Updates(map[string]any{"state": "inactive", "updated_at": now, "error_class": "endpoint_invalid"})
			return
		}
	}
	if err == nil {
		w.db.WithContext(ctx).Model(delivery).Updates(map[string]any{"state": "sent", "sent_at": now, "attempt_count": delivery.AttemptCount + 1, "updated_at": now})
		return
	}
	attempt := delivery.AttemptCount + 1
	state := "retrying"
	next := now.Add(time.Duration(1<<min(attempt, 6)) * time.Minute)
	if attempt >= 6 {
		state = "failed"
	}
	w.db.WithContext(ctx).Model(delivery).Updates(map[string]any{"state": state, "attempt_count": attempt, "next_attempt_at": next, "updated_at": now, "error_class": "provider"})
}
