package migrations

import (
	"fmt"
	"log"

	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db"
	"gorm.io/gorm"
)

func MigrateModels() error {
	dbConnection := db.GetConnection()
	if dbConnection == nil {
		return fmt.Errorf("database connection is not initialized")
	}
	return MigrateModelsWithDB(dbConnection)
}

// MigrateModelsWithDB runs the registered migrations against an explicit
// connection. Production uses MigrateModels; integration tests use this entry
// point so they can verify clean, legacy, partial and concurrent databases
// without mutating the process-global connection.
func MigrateModelsWithDB(dbConnection *gorm.DB) error {
	if dbConnection == nil {
		return fmt.Errorf("database connection is not initialized")
	}

	registry := NewMigrationRegistry(dbConnection)

	RegisterModelMigrations(registry)

	if err := registry.RunAll(); err != nil {
		return err
	}

	log.Println("All migrations completed successfully")
	return nil
}
