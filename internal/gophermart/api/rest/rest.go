package rest

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/kerelape/gophermart/internal/gophermart/api/rest/user"
	"github.com/kerelape/gophermart/internal/gophermart/idp"
)

type REST struct {
	user user.User
}

// New creates a new REST.
func New(idp idp.IdentityProvider) REST {
	return REST{
		user: user.New(idp),
	}
}

func (r REST) Route() http.Handler {
	router := chi.NewRouter()
	router.Mount("/user", r.user.Route())
	return router
}
