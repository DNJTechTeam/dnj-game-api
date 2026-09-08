package db

import (
	"log"
	"net/url"
	"strings"
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

	databaseURL := buildDatabaseURL()

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

	sqlDB.SetMaxIdleConns(1)

	sqlDB.SetMaxOpenConns(2)

	sqlDB.SetConnMaxLifetime(time.Hour)

	SetConnection(db)

	return db
}

// buildDatabaseURL monta a URL de conexão a partir das envs DB_*, removendo
// espaços/quebras de linha acidentais e escapando credenciais com caracteres
// especiais (ex.: "?", "@", "$", "#").
func buildDatabaseURL() string {
	get := func(key string) string { return strings.TrimSpace(common.GetEnv(key)) }

	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(get("DB_USER"), get("DB_PASSWORD")),
		Host:   get("DB_HOST") + ":" + get("DB_PORT"),
		Path:   "/" + get("DB_NAME"),
	}
	return u.String()
}
