package repositories

import (
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
