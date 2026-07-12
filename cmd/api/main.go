package main

import (
	"github.com/dnjtechteam/dnj-game-api/cmd/api/di"
	apiRunner "github.com/dnjtechteam/dnj-game-api/internal/infrastructure/api"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/common"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db"

	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()

	db.Init()

	api := di.InitializeServer()

	router := api.Router.RegisterRoutes()

	apiRunner.Run(router)

	common.WaitOsInterruption()
}
