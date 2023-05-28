package user

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/kerelape/gophermart/internal/gophermart/api/rest/user/register"
	"github.com/kerelape/gophermart/internal/gophermart/idp"
)

type User struct {
	register register.Register
}

func New(idp idp.IdentityProvider) User {
	return User{
		register: register.New(idp),
	}
}

func (u User) Route() http.Handler {
	router := chi.NewRouter()
	router.Mount("/register", u.register.Route())
	return router
}
