package gophermart

import (
	"context"
	"github.com/kerelape/gophermart/internal/accrual"
	"github.com/kerelape/gophermart/internal/gophermart/api"
	"github.com/kerelape/gophermart/internal/gophermart/idp"
	"github.com/pior/runnable"
	"net/http"
)

type Gophermart struct {
	config Config
}

// New creates a new Gophermart.
func New(config Config) Gophermart {
	return Gophermart{
		config: config,
	}
}

func (g Gophermart) Run(ctx context.Context) error {
	database := idp.NewPostgresIdentityDatabase(
		g.config.AddressDatabase,
		accrual.New(
			g.config.AddressAccrualSystem,
			http.DefaultClient,
		),
	)
	identityProvider := idp.NewBearerIdentityProvider(database, []byte("top-secret"))
	apiService := api.New(identityProvider, g.config.AddressAPIServer)

	manager := runnable.NewManager()
	manager.Add(database)
	manager.Add(apiService)
	return manager.Build().Run(ctx)
}
