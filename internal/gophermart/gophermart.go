package gophermart

import (
	"context"
	"github.com/kerelape/gophermart/internal/accrual"
	"github.com/kerelape/gophermart/internal/gophermart/api"
	"github.com/kerelape/gophermart/internal/gophermart/idp"
	"github.com/pior/runnable"
	"net/http"
	"time"
)

type Gophermart struct {
	addressAPIServer     string
	addressAccrualSystem string
	addressDatabase      string
	jwtSecret            string
}

// New creates a new Gophermart.
func New(addressAPIServer, addressAccrualSystem, addressDatabase, jwtSecret string) Gophermart {
	return Gophermart{
		addressAPIServer:     addressAPIServer,
		addressAccrualSystem: addressAccrualSystem,
		addressDatabase:      addressDatabase,
		jwtSecret:            jwtSecret,
	}
}

func (g Gophermart) Run(ctx context.Context) error {
	acc := accrual.New(g.addressAccrualSystem, http.DefaultClient)
	database := idp.NewPostgresIdentityDatabase(g.addressDatabase, acc)
	identityProvider := idp.NewBearerIdentityProvider(database, []byte(g.jwtSecret))
	apiService := api.New(identityProvider, g.addressAPIServer)

	manager := runnable.NewManager()
	manager.Add(database)
	manager.Add(runnable.Every(idp.NewUpdateOrdersTask(acc, database), time.Second))
	manager.Add(apiService)
	return manager.Build().Run(ctx)
}
