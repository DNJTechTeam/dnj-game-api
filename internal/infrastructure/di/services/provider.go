package services

import (
	appInterfaces "github.com/dnjtechteam/dnj-game-api/internal/app/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/app/services"
	activityInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/activity/interfaces"
	adminInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/adminoperation/interfaces"
	commonInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/common/interfaces"
	favoriteInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/favorite/interfaces"
	groupInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/group/interfaces"
	inviteInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/groupinvite/interfaces"
	membershipInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/groupmembership/interfaces"
	identityInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/identity/interfaces"
	auditInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/operationaudit/interfaces"
	refreshInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/refreshsession/interfaces"
	spaceInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/space/interfaces"
	swInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/subscriptionwebhook/interfaces"
	svcInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/subscriptionwebhookverificationcode/interfaces"
	taskInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/task/interfaces"
	uInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/user/interfaces"
	emailServicePkg "github.com/dnjtechteam/dnj-game-api/internal/infrastructure/email"
	googlePkg "github.com/dnjtechteam/dnj-game-api/internal/infrastructure/google"
	webhookPkg "github.com/dnjtechteam/dnj-game-api/internal/infrastructure/webhook"
)

func ProvideAdminInstallationService(baseService *services.BaseService, spaceRepository spaceInterfaces.SpaceRepositoryInterface, activityRepository activityInterfaces.ActivityRepositoryInterface, auditRepository auditInterfaces.OperationAuditRepositoryInterface, adminOperationRepository adminInterfaces.AdminOperationRepositoryInterface, userRepository uInterfaces.UserRepositoryInterface) appInterfaces.AdminInstallationServiceInterface {
	return services.NewAdminInstallationService(baseService, spaceRepository, activityRepository, auditRepository, adminOperationRepository, userRepository)
}

func ProvideSpaceService(spaceRepository spaceInterfaces.SpaceRepositoryInterface) appInterfaces.SpaceServiceInterface {
	return services.NewSpaceService(spaceRepository)
}

func ProvideActivityService(baseService *services.BaseService, activityRepository activityInterfaces.ActivityRepositoryInterface, auditRepository auditInterfaces.OperationAuditRepositoryInterface, userRepository uInterfaces.UserRepositoryInterface) appInterfaces.ActivityServiceInterface {
	return services.NewActivityService(baseService, activityRepository, auditRepository, userRepository)
}

func ProvideContentService(activityRepository activityInterfaces.ActivityRepositoryInterface, spaceRepository spaceInterfaces.SpaceRepositoryInterface) appInterfaces.ContentServiceInterface {
	return services.NewContentService(activityRepository, spaceRepository)
}

func ProvideFavoriteService(baseService *services.BaseService, favoriteRepository favoriteInterfaces.FavoriteRepositoryInterface, activityRepository activityInterfaces.ActivityRepositoryInterface, userRepository uInterfaces.UserRepositoryInterface) appInterfaces.FavoriteServiceInterface {
	return services.NewFavoriteService(baseService, favoriteRepository, activityRepository, userRepository)
}

func ProvideBaseService(transactionManager commonInterfaces.TransactionManagerInterface) *services.BaseService {
	return services.NewBaseService(transactionManager)
}

func ProvideJwtService(baseService *services.BaseService) appInterfaces.JwtServiceInterface {
	return services.NewJwtService(baseService)
}

func ProvideEmailService() appInterfaces.EmailServiceInterface {
	return emailServicePkg.NewEmailService()
}

func ProvideGoogleIDTokenVerifier() appInterfaces.GoogleIDTokenVerifierInterface {
	return googlePkg.NewIDTokenVerifier()
}

func ProvideIdentityService(
	baseService *services.BaseService,
	userRepository uInterfaces.UserRepositoryInterface,
	groupRepository groupInterfaces.GroupRepositoryInterface,
	membershipRepository membershipInterfaces.GroupMembershipRepositoryInterface,
	identityRepository identityInterfaces.GoogleIdentityRepositoryInterface,
	refreshRepository refreshInterfaces.RefreshSessionRepositoryInterface,
	jwtService appInterfaces.JwtServiceInterface,
	googleVerifier appInterfaces.GoogleIDTokenVerifierInterface,
) appInterfaces.IdentityServiceInterface {
	return services.NewIdentityService(baseService, userRepository, groupRepository, membershipRepository, identityRepository, refreshRepository, jwtService, googleVerifier)
}

func ProvideWebhookPayloadTranslator() appInterfaces.WebhookPayloadTranslatorInterface {
	return webhookPkg.NewOrderPayloadTranslator()
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
	groupRepository groupInterfaces.GroupRepositoryInterface,
	translator appInterfaces.WebhookPayloadTranslatorInterface,
) appInterfaces.SubscriptionWebhookServiceInterface {
	return services.NewSubscriptionWebhookService(baseService, subscriptionWebhookRepository, verificationCodeRepository, groupRepository, translator)
}

func ProvideAuthService(
	baseService *services.BaseService,
	verificationCodeRepository svcInterfaces.SubscriptionWebhookVerificationCodeRepositoryInterface,
	userRepository uInterfaces.UserRepositoryInterface,
	groupRepository groupInterfaces.GroupRepositoryInterface,
	membershipRepository membershipInterfaces.GroupMembershipRepositoryInterface,
	jwtService appInterfaces.JwtServiceInterface,
	emailService appInterfaces.EmailServiceInterface,
) appInterfaces.AuthServiceInterface {
	return services.NewAuthService(baseService, verificationCodeRepository, userRepository, groupRepository, membershipRepository, jwtService, emailService)
}

func ProvideGroupService(
	baseService *services.BaseService,
	groupRepository groupInterfaces.GroupRepositoryInterface,
	userRepository uInterfaces.UserRepositoryInterface,
	membershipRepository membershipInterfaces.GroupMembershipRepositoryInterface,
) appInterfaces.GroupServiceInterface {
	return services.NewGroupService(baseService, groupRepository, userRepository, membershipRepository)
}

func ProvideProfileService(baseService *services.BaseService, userRepository uInterfaces.UserRepositoryInterface, groupRepository groupInterfaces.GroupRepositoryInterface) appInterfaces.ProfileServiceInterface {
	return services.NewProfileService(baseService, userRepository, groupRepository)
}

func ProvideGroupInviteService(baseService *services.BaseService, userRepository uInterfaces.UserRepositoryInterface, groupRepository groupInterfaces.GroupRepositoryInterface, membershipRepository membershipInterfaces.GroupMembershipRepositoryInterface, inviteRepository inviteInterfaces.GroupInviteRepositoryInterface) appInterfaces.GroupInviteServiceInterface {
	return services.NewGroupInviteService(baseService, userRepository, groupRepository, membershipRepository, inviteRepository)
}

func ProvideUserService(
	baseService *services.BaseService,
	userRepository uInterfaces.UserRepositoryInterface,
	groupRepository groupInterfaces.GroupRepositoryInterface,
	membershipRepository membershipInterfaces.GroupMembershipRepositoryInterface,
) appInterfaces.UserServiceInterface {
	return services.NewUserService(baseService, userRepository, groupRepository, membershipRepository)
}
