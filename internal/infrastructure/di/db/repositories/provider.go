package repositories

import (
	groupInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/group/interfaces"
	swInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/subscriptionwebhook/interfaces"
	svcInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/subscriptionwebhookverificationcode/interfaces"
	taskInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/task/interfaces"
	uInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/user/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/repositories"

	"gorm.io/gorm"
)

func ProvideUserRepository(db *gorm.DB) uInterfaces.UserRepositoryInterface {
	return repositories.NewUserRepository(db)
}

func ProvideTaskRepository(db *gorm.DB) taskInterfaces.TaskRepositoryInterface {
	return repositories.NewTaskRepository(db)
}

func ProvideGroupRepository(db *gorm.DB) groupInterfaces.GroupRepositoryInterface {
	return repositories.NewGroupRepository(db)
}

func ProvideSubscriptionWebhookRepository(db *gorm.DB) swInterfaces.SubscriptionWebhookRepositoryInterface {
	return repositories.NewSubscriptionWebhookRepository(db)
}

func ProvideSubscriptionWebhookVerificationCodeRepository(db *gorm.DB) svcInterfaces.SubscriptionWebhookVerificationCodeRepositoryInterface {
	return repositories.NewSubscriptionWebhookVerificationCodeRepository(db)
}
