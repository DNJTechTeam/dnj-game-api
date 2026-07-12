package db

import (
	commonInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/common/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db"

	"gorm.io/gorm"
)

func ProvideTransactionManager(gormDB *gorm.DB) commonInterfaces.TransactionManagerInterface {
	return db.NewTransactionManager(gormDB)
}
