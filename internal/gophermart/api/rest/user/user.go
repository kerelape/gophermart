package user

import (
	"github.com/kerelape/gophermart/internal/gophermart/api/rest/user/orders"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/kerelape/gophermart/internal/gophermart/api/rest/user/login"
	"github.com/kerelape/gophermart/internal/gophermart/api/rest/user/register"
	"github.com/kerelape/gophermart/internal/gophermart/idp"
)

type User struct {
	register register.Register
	login    login.Login
	orders   orders.Orders
}

// New creates a new User.
func New(idp idp.IdentityProvider) User {
	return User{
		register: register.New(idp),
		login:    login.New(idp),
		orders:   orders.New(idp),
	}
}

func (u User) Route() http.Handler {
	router := chi.NewRouter()
	router.Mount("/register", u.register.Route())
	router.Mount("/login", u.login.Route())
	router.Mount("/ordres", u.orders.Route())
	return router
}
