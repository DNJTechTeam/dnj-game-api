// @title           DNJ Game API
// @version         1.0
// @description     API REST para gerenciamento de eventos e pontuações da gamificação DNJ.
// @BasePath        /v1
//
// @securityDefinitions.apikey  BearerAuth
// @in                          header
// @name                        Authorization
// @description                Identity token (JWT). Também aceito via cookie identity_token; o header é a forma documentada aqui.
package main

import (
	"github.com/dnjtechteam/dnj-game-api/cmd/api/di"
	apiRunner "github.com/dnjtechteam/dnj-game-api/internal/infrastructure/api"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/common"

	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()

	api := di.InitializeServer()

	router := api.Router.RegisterRoutes()

	apiRunner.Run(router)

	common.WaitOsInterruption()
}
