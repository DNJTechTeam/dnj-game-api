package api

import "github.com/scienceandcode/nucleus-api/internal/presentation/api/routers"

type API struct {
	Router *routers.Router
}

func NewAPI(router *routers.Router) *API {
	return &API{
		Router: router,
	}
}
