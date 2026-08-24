//go:build wireinject
// +build wireinject

package di

import (
	diApi "github.com/dnjtechteam/dnj-game-api/internal/infrastructure/di/api"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/di/db"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/di/db/repositories"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/di/services"
	"github.com/dnjtechteam/dnj-game-api/internal/presentation/api"
	"github.com/dnjtechteam/dnj-game-api/internal/presentation/api/handlers"

	"github.com/google/wire"
)

func InitializeServer() *api.API {
	wire.Build(
		db.ProvideDB,
		db.ProvideTransactionManager,

		repositories.ProvideUserRepository,
		repositories.ProvideGoogleIdentityRepository,
		repositories.ProvideRefreshSessionRepository,
		repositories.ProvideTaskRepository,
		repositories.ProvideGroupRepository,
		repositories.ProvideGroupMembershipRepository,
		repositories.ProvideGroupInviteRepository,
		repositories.ProvideSpaceRepository,
		repositories.ProvideActivityRepository,
		repositories.ProvideFavoriteRepository,
		repositories.ProvideGameRepository,
		repositories.ProvideOperationAuditRepository,
		repositories.ProvideAdminOperationRepository,
		repositories.ProvideSubscriptionWebhookRepository,
		repositories.ProvideSubscriptionWebhookVerificationCodeRepository,

		services.ProvideBaseService,
		services.ProvideJwtService,
		services.ProvideGoogleIDTokenVerifier,
		services.ProvideIdentityService,
		services.ProvideEmailService,
		services.ProvideWebhookPayloadTranslator,
		services.ProvideTaskService,
		services.ProvideSubscriptionWebhookService,
		services.ProvideAuthService,
		services.ProvideGroupService,
		services.ProvideProfileService,
		services.ProvideGroupInviteService,
		services.ProvideUserService,
		services.ProvideSpaceService,
		services.ProvideActivityService,
		services.ProvideContentService,
		services.ProvideFavoriteService,
		services.ProvideGameService,
		services.ProvideAdminInstallationService,

		diApi.ProvideEngine,
		diApi.ProvideRouter,

		wire.Struct(new(handlers.HealthcheckHandler), "*"),
		wire.Struct(new(handlers.AuthHandler), "*"),
		wire.Struct(new(handlers.IdentityHandler), "*"),
		wire.Struct(new(handlers.ProfileHandler), "*"),
		wire.Struct(new(handlers.GroupInviteHandler), "*"),
		wire.Struct(new(handlers.SubscriptionWebhookHandler), "*"),
		wire.Struct(new(handlers.GroupHandler), "*"),
		wire.Struct(new(handlers.UserHandler), "*"),
		wire.Struct(new(handlers.TaskHandler), "*"),
		wire.Struct(new(handlers.InstallationHandler), "*"),
		wire.Struct(new(handlers.AdminInstallationHandler), "*"),
		wire.Struct(new(handlers.ContentHandler), "*"),
		wire.Struct(new(handlers.FavoriteHandler), "*"),
		wire.Struct(new(handlers.GameHandler), "*"),
		wire.Struct(new(handlers.Handlers), "*"),
		wire.Struct(new(api.API), "*"),
	)

	return nil
}
