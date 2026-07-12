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
		repositories.ProvideTaskRepository,

		services.ProvideBaseService,
		services.ProvideJwtService,
		services.ProvideEmailService,
		services.ProvideEmailConfirmationService,
		services.ProvideUserAuthService,
		services.ProvideTaskService,

		diApi.ProvideEngine,
		diApi.ProvideRouter,

		wire.Struct(new(handlers.HealthcheckHandler), "*"),
		wire.Struct(new(handlers.UserAuthHandler), "*"),
		wire.Struct(new(handlers.TaskHandler), "*"),
		wire.Struct(new(handlers.Handlers), "*"),
		wire.Struct(new(api.API), "*"),
	)

	return nil
}
