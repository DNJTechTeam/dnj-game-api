package interfaces

import (
	"context"
	"time"

	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	"github.com/dnjtechteam/dnj-game-api/internal/domain/activity/entities"
	userEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
)

type ActivityRepositoryInterface interface {
	FindAuthorizedForUpdate(ctx context.Context, activityID string, actorUserID uint64, global bool) (*entities.Activity, error)
	TransitionStatus(ctx context.Context, activityID string, from, to entities.Status, updatedAt time.Time) error
	List(ctx context.Context, page uint64) (*messages.PaginatedResponse[entities.Activity], error)
	Create(ctx context.Context, activity *entities.Activity) (*entities.Activity, error)
	FindByID(ctx context.Context, activityID string) (*entities.Activity, error)
	FindByIDForUpdate(ctx context.Context, activityID string) (*entities.Activity, error)
	Update(ctx context.Context, activity *entities.Activity) (*entities.Activity, error)
	ListManagers(ctx context.Context, activityID string, page uint64) (*messages.PaginatedResponse[userEntities.User], error)
	CreateManagerAssignment(ctx context.Context, assignment *entities.ManagerAssignment) (bool, error)
	DeleteManagerAssignment(ctx context.Context, activityID string, userID uint64) (bool, error)
	CountManagerAssignments(ctx context.Context, userID uint64) (int64, error)
	ListSchedule(ctx context.Context, sectorSlug string, limit int) ([]entities.PublicActivity, error)
	ListManagerSchedule(ctx context.Context, actorUserID uint64, global bool) ([]entities.PublicActivity, error)
	ListPublic(ctx context.Context, kind *entities.Kind, spaceID *string, generatedAt time.Time, page uint64) (*messages.PaginatedResponse[entities.PublicActivity], error)
	FindPublicByID(ctx context.Context, activityID string, generatedAt time.Time) (*entities.PublicActivity, error)
}
