package api

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/kerelape/gophermart/internal/gophermart/api/rest"
	"github.com/kerelape/gophermart/internal/gophermart/idp"
	"github.com/pior/runnable"
)

type API struct {
	rest rest.REST

	ServerAddress string
}

// New creates a new API.
func New(idp idp.IdentityProvider, address string) API {
	return API{
		rest: rest.New(idp),

		ServerAddress: address,
	}
}

func (a API) Run(ctx context.Context) error {
	router := chi.NewRouter().Group(func(router chi.Router) {
		router.Mount("/api", a.rest.Route())
	})
	server := http.Server{
		Addr:    a.ServerAddress,
		Handler: router,
	}
	manager := runnable.NewManager()
	manager.Add(runnable.HTTPServer(&server))
	return manager.Build().Run(ctx)
}
