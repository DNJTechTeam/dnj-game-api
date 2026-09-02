package interfaces

import (
	"context"
	"time"

	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	"github.com/dnjtechteam/dnj-game-api/internal/domain/notification/entities"
)

type Repository interface {
	// FindPreferences returns appErrors.ErrNotFound when the user never
	// customized their preferences; callers apply the enabled-by-default rule.
	FindPreferences(ctx context.Context, userID uint64) (*entities.Preferences, error)
	UpsertPreferences(ctx context.Context, prefs *entities.Preferences) (*entities.Preferences, error)

	List(ctx context.Context, userID uint64, page uint64) (*messages.PaginatedResponse[entities.Notification], error)
	CountUnread(ctx context.Context, userID uint64) (uint64, error)
	FindByIDAndUser(ctx context.Context, id string, userID uint64) (*entities.Notification, error)
	// MarkRead is idempotent: marking an already-read notification returns it
	// unchanged and never reverts state back to unread.
	MarkRead(ctx context.Context, id string, userID uint64, now time.Time) (*entities.Notification, error)

	// ResolveAnnouncementRecipients returns the DEFAULT, onboarded user IDs
	// eligible for an administrative announcement, honoring their
	// announcement preference. When explicitUserIDs is non-empty it narrows
	// the broadcast to that set instead of every eligible user.
	ResolveAnnouncementRecipients(ctx context.Context, explicitUserIDs []uint64) ([]uint64, error)
	CreateBroadcast(ctx context.Context, notifications []*entities.Notification) error
	UpsertPushSubscription(ctx context.Context, subscription *entities.PushSubscription) (*entities.PushSubscription, error)
	DeactivatePushSubscription(ctx context.Context, userID uint64, endpoint string, now time.Time) error
	CreateQueueCall(ctx context.Context, notification *entities.Notification, now time.Time) (bool, error)

	FindOperation(ctx context.Context, actorID uint64, key string) (*entities.Operation, error)
	CreateOperation(ctx context.Context, operation *entities.Operation) error
}
