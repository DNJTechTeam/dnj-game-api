package services

import (
	appInterfaces "github.com/dnjtechteam/dnj-game-api/internal/app/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/app/services"
	commonInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/common/interfaces"
	groupInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/group/interfaces"
	swInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/subscriptionwebhook/interfaces"
	svcInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/subscriptionwebhookverificationcode/interfaces"
	taskInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/task/interfaces"
	uInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/user/interfaces"
	emailServicePkg "github.com/dnjtechteam/dnj-game-api/internal/infrastructure/email"
	webhookPkg "github.com/dnjtechteam/dnj-game-api/internal/infrastructure/webhook"
)

func ProvideBaseService(transactionManager commonInterfaces.TransactionManagerInterface) *services.BaseService {
	return services.NewBaseService(transactionManager)
}

func ProvideJwtService(baseService *services.BaseService) appInterfaces.JwtServiceInterface {
	return services.NewJwtService(baseService)
}

func ProvideEmailService() appInterfaces.EmailServiceInterface {
	return emailServicePkg.NewEmailService()
}

func ProvideWebhookPayloadTranslator() appInterfaces.WebhookPayloadTranslatorInterface {
	return webhookPkg.NewNaivePayloadTranslator()
}

func ProvideTaskService(
	baseService *services.BaseService,
	taskRepository taskInterfaces.TaskRepositoryInterface,
) appInterfaces.TaskServiceInterface {
	return services.NewTaskService(baseService, taskRepository)
}

func ProvideSubscriptionWebhookService(
	baseService *services.BaseService,
	subscriptionWebhookRepository swInterfaces.SubscriptionWebhookRepositoryInterface,
	verificationCodeRepository svcInterfaces.SubscriptionWebhookVerificationCodeRepositoryInterface,
	translator appInterfaces.WebhookPayloadTranslatorInterface,
) appInterfaces.SubscriptionWebhookServiceInterface {
	return services.NewSubscriptionWebhookService(baseService, subscriptionWebhookRepository, verificationCodeRepository, translator)
}

func ProvideAuthService(
	baseService *services.BaseService,
	verificationCodeRepository svcInterfaces.SubscriptionWebhookVerificationCodeRepositoryInterface,
	userRepository uInterfaces.UserRepositoryInterface,
	groupRepository groupInterfaces.GroupRepositoryInterface,
	jwtService appInterfaces.JwtServiceInterface,
	emailService appInterfaces.EmailServiceInterface,
) appInterfaces.AuthServiceInterface {
	return services.NewAuthService(baseService, verificationCodeRepository, userRepository, groupRepository, jwtService, emailService)
}

func ProvideGroupService(
	baseService *services.BaseService,
	groupRepository groupInterfaces.GroupRepositoryInterface,
) appInterfaces.GroupServiceInterface {
	return services.NewGroupService(baseService, groupRepository)
}

func ProvideUserService(
	baseService *services.BaseService,
	userRepository uInterfaces.UserRepositoryInterface,
	groupRepository groupInterfaces.GroupRepositoryInterface,
) appInterfaces.UserServiceInterface {
	return services.NewUserService(baseService, userRepository, groupRepository)
}
