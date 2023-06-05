package user

import (
	"github.com/kerelape/gophermart/internal/gophermart/api/rest/authorization"
	"github.com/kerelape/gophermart/internal/gophermart/api/rest/user/balance"
	"github.com/kerelape/gophermart/internal/gophermart/api/rest/user/orders"
	"github.com/kerelape/gophermart/internal/gophermart/api/rest/user/withdrawals"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/kerelape/gophermart/internal/gophermart/api/rest/user/login"
	"github.com/kerelape/gophermart/internal/gophermart/api/rest/user/register"
	"github.com/kerelape/gophermart/internal/gophermart/idp"
)

type User struct {
	register    register.Register
	login       login.Login
	orders      orders.Orders
	balance     balance.Balance
	withdrawals withdrawals.Withdrawals

	identityProvider idp.IdentityProvider
}

// New creates a new User.
func New(identityProvider idp.IdentityProvider) User {
	return User{
		register:    register.New(identityProvider),
		login:       login.New(identityProvider),
		orders:      orders.New(),
		balance:     balance.New(),
		withdrawals: withdrawals.New(),

		identityProvider: identityProvider,
	}
}

func (u User) Route() http.Handler {
	router := chi.NewRouter()
	router.Mount("/register", u.register.Route())
	router.Mount("/login", u.login.Route())
	router.Group(func(router chi.Router) {
		router.Use(authorization.Authorization(u.identityProvider))
		router.Mount("/orders", u.orders.Route())
		router.Mount("/balance", u.balance.Route())
		router.Mount("/withdrawals", u.withdrawals.Route())
	})
	return router
}
