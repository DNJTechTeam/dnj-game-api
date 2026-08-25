package db

import (
	"fmt"
	"log"
	"time"

	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/common"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var gormDbConnection *gorm.DB

func GetConnection() *gorm.DB {
	return gormDbConnection
}

func SetConnection(db *gorm.DB) {
	gormDbConnection = db
}

func Init() *gorm.DB {
	return initConnection(false)
}

// InitAPI builds the pool without an eager ping. This allows the HTTP process
// to expose liveness and a deterministic readiness=503 when PostgreSQL is
// temporarily unavailable. Migration commands keep using Init, which remains
// fail-fast and blocks unsafe deploys.
func InitAPI() *gorm.DB {
	return initConnection(true)
}

func initConnection(disableAutomaticPing bool) *gorm.DB {
	if gormDbConnection != nil {
		return gormDbConnection
	}

	user := common.GetEnv("DB_USER")
	password := common.GetEnv("DB_PASSWORD")
	host := common.GetEnv("DB_HOST")
	port := common.GetEnv("DB_PORT")
	dbname := common.GetEnv("DB_NAME")

	databaseURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s", user, password, host, port, dbname)

	config := &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		DisableAutomaticPing:                     disableAutomaticPing,
	}
	if !common.EnvironmentIs(common.EnvironmentLocalhost) {
		config.Logger = logger.Default.LogMode(logger.Silent)
	}

	db, err := gorm.Open(postgres.Open(databaseURL), config)

	if err != nil {
		log.Fatalln(err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalln("Failed to get underlying sql.DB:", err)
	}

	sqlDB.SetMaxIdleConns(10)

	sqlDB.SetMaxOpenConns(100)

	sqlDB.SetConnMaxLifetime(time.Hour)

	SetConnection(db)

	return db
}
