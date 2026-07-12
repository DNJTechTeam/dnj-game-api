package main

import (
	"log"

	"github.com/scienceandcode/nucleus-api/cmd/api/di"
	apiRunner "github.com/scienceandcode/nucleus-api/internal/infrastructure/api"
	"github.com/scienceandcode/nucleus-api/internal/infrastructure/common"
	"github.com/scienceandcode/nucleus-api/internal/infrastructure/db"
	"github.com/scienceandcode/nucleus-api/internal/infrastructure/db/migrations"

	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()

	db.Init()
	if err := migrations.MigrateModels(); err != nil {
		log.Fatalf("failed to run database migrations: %v", err)
	}

	api := di.InitializeServer()

	router := api.Router.RegisterRoutes()

	apiRunner.Run(router)

	common.WaitOsInterruption()
}
