package gophermart

import (
	"context"

	"github.com/kerelape/gophermart/internal/gophermart/api"
	"github.com/pior/runnable"
)

type Gophermart struct {
	api api.API
}

func New(config Config) Gophermart {
	return Gophermart{
		api.New(config.ServerAddress),
	}
}

func (g Gophermart) Run(ctx context.Context) error {
	manager := runnable.NewManager()
	manager.Add(g.api)
	return manager.Build().Run(ctx)
}
