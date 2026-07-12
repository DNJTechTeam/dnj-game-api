package repositories

import (
	taskInterfaces "github.com/scienceandcode/nucleus-api/internal/domain/task/interfaces"
	uInterfaces "github.com/scienceandcode/nucleus-api/internal/domain/user/interfaces"
	"github.com/scienceandcode/nucleus-api/internal/infrastructure/db/repositories"

	"gorm.io/gorm"
)

func ProvideUserRepository(db *gorm.DB) uInterfaces.UserRepositoryInterface {
	return repositories.NewUserRepository(db)
}

func ProvideTaskRepository(db *gorm.DB) taskInterfaces.TaskRepositoryInterface {
	return repositories.NewTaskRepository(db)
}
