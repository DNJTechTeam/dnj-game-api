package db

import (
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db"

	"gorm.io/gorm"
)

func ProvideDB() *gorm.DB {
	return db.InitAPI()
}
