package rest

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/kerelape/gophermart/internal/gophermart/api/rest/user"
)

type REST struct {
	user user.User
}

// New creates a new REST.
func New() REST {
	return REST{
		user: user.New(),
	}
}

func (r REST) Route() http.Handler {
	router := chi.NewRouter()
	router.Mount("/user", r.user.Route())
	return router
}
