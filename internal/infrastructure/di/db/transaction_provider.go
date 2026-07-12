package db

import (
	commonInterfaces "github.com/scienceandcode/nucleus-api/internal/domain/common/interfaces"
	"github.com/scienceandcode/nucleus-api/internal/infrastructure/db"

	"gorm.io/gorm"
)

func ProvideTransactionManager(gormDB *gorm.DB) commonInterfaces.TransactionManagerInterface {
	return db.NewTransactionManager(gormDB)
}
