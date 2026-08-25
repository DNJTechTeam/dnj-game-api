package repositories

import (
	activityInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/activity/interfaces"
	adminInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/adminoperation/interfaces"
	favoriteInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/favorite/interfaces"
	gameInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/game/interfaces"
	groupInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/group/interfaces"
	inviteInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/groupinvite/interfaces"
	membershipInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/groupmembership/interfaces"
	identityInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/identity/interfaces"
	mediaInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/media/interfaces"
	momentInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/moment/interfaces"
	notificationInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/notification/interfaces"
	auditInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/operationaudit/interfaces"
	refreshInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/refreshsession/interfaces"
	spaceInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/space/interfaces"
	swInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/subscriptionwebhook/interfaces"
	svcInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/subscriptionwebhookverificationcode/interfaces"
	taskInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/task/interfaces"
	uInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/user/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/repositories"

	"gorm.io/gorm"
)

func ProvideAdminOperationRepository(db *gorm.DB) adminInterfaces.AdminOperationRepositoryInterface {
	return repositories.NewAdminOperationRepository(db)
}

func ProvideSpaceRepository(db *gorm.DB) spaceInterfaces.SpaceRepositoryInterface {
	return repositories.NewSpaceRepository(db)
}

func ProvideActivityRepository(db *gorm.DB) activityInterfaces.ActivityRepositoryInterface {
	return repositories.NewActivityRepository(db)
}

func ProvideFavoriteRepository(db *gorm.DB) favoriteInterfaces.FavoriteRepositoryInterface {
	return repositories.NewFavoriteRepository(db)
}

func ProvideGameRepository(db *gorm.DB) gameInterfaces.GameRepositoryInterface {
	return repositories.NewGameRepository(db)
}

func ProvideMediaRepository(db *gorm.DB) mediaInterfaces.Repository {
	return repositories.NewMediaRepository(db)
}

func ProvideMomentRepository(db *gorm.DB) momentInterfaces.Repository {
	return repositories.NewMomentRepository(db)
}

func ProvideNotificationRepository(db *gorm.DB) notificationInterfaces.Repository {
	return repositories.NewNotificationRepository(db)
}

func ProvideOperationAuditRepository(db *gorm.DB) auditInterfaces.OperationAuditRepositoryInterface {
	return repositories.NewOperationAuditRepository(db)
}

func ProvideUserRepository(db *gorm.DB) uInterfaces.UserRepositoryInterface {
	return repositories.NewUserRepository(db)
}

func ProvideGoogleIdentityRepository(db *gorm.DB) identityInterfaces.GoogleIdentityRepositoryInterface {
	return repositories.NewGoogleIdentityRepository(db)
}

func ProvideRefreshSessionRepository(db *gorm.DB) refreshInterfaces.RefreshSessionRepositoryInterface {
	return repositories.NewRefreshSessionRepository(db)
}

func ProvideTaskRepository(db *gorm.DB) taskInterfaces.TaskRepositoryInterface {
	return repositories.NewTaskRepository(db)
}

func ProvideGroupRepository(db *gorm.DB) groupInterfaces.GroupRepositoryInterface {
	return repositories.NewGroupRepository(db)
}

func ProvideGroupMembershipRepository(db *gorm.DB) membershipInterfaces.GroupMembershipRepositoryInterface {
	return repositories.NewGroupMembershipRepository(db)
}

func ProvideGroupInviteRepository(db *gorm.DB) inviteInterfaces.GroupInviteRepositoryInterface {
	return repositories.NewGroupInviteRepository(db)
}

func ProvideSubscriptionWebhookRepository(db *gorm.DB) swInterfaces.SubscriptionWebhookRepositoryInterface {
	return repositories.NewSubscriptionWebhookRepository(db)
}

func ProvideSubscriptionWebhookVerificationCodeRepository(
	db *gorm.DB,
) svcInterfaces.SubscriptionWebhookVerificationCodeRepositoryInterface {
	return repositories.NewSubscriptionWebhookVerificationCodeRepository(db)
}
