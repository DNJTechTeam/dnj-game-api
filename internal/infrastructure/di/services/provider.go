package services

import (
	appInterfaces "github.com/dnjtechteam/dnj-game-api/internal/app/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/app/services"
	commonInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/common/interfaces"
	taskInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/task/interfaces"
	uInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/user/interfaces"
	emailServicePkg "github.com/dnjtechteam/dnj-game-api/internal/infrastructure/email"
)

func ProvideBaseService(transactionManager commonInterfaces.TransactionManagerInterface) *services.BaseService {
	return services.NewBaseService(transactionManager)
}

func ProvideJwtService(baseService *services.BaseService) appInterfaces.JwtServiceInterface {
	return services.NewJwtService(baseService)
}

func ProvideEmailConfirmationService(baseService *services.BaseService) appInterfaces.EmailConfirmationServiceInterface {
	return services.NewEmailConfirmationService(baseService)
}

func ProvideEmailService() appInterfaces.EmailServiceInterface {
	return emailServicePkg.NewEmailService()
}

func ProvideUserAuthService(
	baseService *services.BaseService,
	userRepository uInterfaces.UserRepositoryInterface,
	jwtService appInterfaces.JwtServiceInterface,
	emailService appInterfaces.EmailServiceInterface,
	emailConfirmationService appInterfaces.EmailConfirmationServiceInterface,
) appInterfaces.UserAuthServiceInterface {
	return services.NewUserAuthService(baseService, userRepository, jwtService, emailService, emailConfirmationService)
}

func ProvideTaskService(
	baseService *services.BaseService,
	taskRepository taskInterfaces.TaskRepositoryInterface,
) appInterfaces.TaskServiceInterface {
	return services.NewTaskService(baseService, taskRepository)
}
