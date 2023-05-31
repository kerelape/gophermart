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
	addressAPIServer     string
	addressAccrualSystem string
	addressDatabase      string
}

// New creates a new Gophermart.
func New(addressAPIServer, addressAccrualSystem, addressDatabase string) Gophermart {
	return Gophermart{
		addressAPIServer:     addressAPIServer,
		addressAccrualSystem: addressAccrualSystem,
		addressDatabase:      addressDatabase,
	}
}

func (g Gophermart) Run(ctx context.Context) error {
	database := idp.NewPostgresIdentityDatabase(
		g.addressDatabase,
		accrual.New(
			g.addressAccrualSystem,
			http.DefaultClient,
		),
	)
	identityProvider := idp.NewBearerIdentityProvider(database, []byte("top-secret"))
	apiService := api.New(identityProvider, g.addressAPIServer)

	manager := runnable.NewManager()
	manager.Add(database)
	manager.Add(apiService)
	return manager.Build().Run(ctx)
}
