package db

import (
	"github.com/scienceandcode/nucleus-api/internal/infrastructure/db"

	"gorm.io/gorm"
)

func ProvideDB() *gorm.DB {
	return db.Init()
}
