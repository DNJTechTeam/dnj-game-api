package main

import (
	"log"

	"github.com/joho/godotenv"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/migrations"
)

func main() {
	godotenv.Load()

	db.Init()

	log.Println("Running migrations...")
	if err := migrations.MigrateModels(); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}
	log.Println("Migrations completed successfully.")
}
