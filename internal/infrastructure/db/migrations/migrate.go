package migrations

import (
	"fmt"
	"log"

	"github.com/scienceandcode/nucleus-api/internal/infrastructure/db"
)

func MigrateModels() error {
	dbConnection := db.GetConnection()
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
