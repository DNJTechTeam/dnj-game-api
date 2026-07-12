package api

import "github.com/dnjtechteam/dnj-game-api/internal/presentation/api/routers"

type API struct {
	Router *routers.Router
}

func NewAPI(router *routers.Router) *API {
	return &API{
		Router: router,
	}
}
