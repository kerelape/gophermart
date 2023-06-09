package login

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/kerelape/gophermart/internal/gophermart/idp"
)

type Login struct {
	IdentityProvider idp.IdentityProvider
}

// New creates a new Login.
func New(identityProvider idp.IdentityProvider) Login {
	return Login{
		IdentityProvider: identityProvider,
	}
}

func (l Login) Route() http.Handler {
	router := chi.NewRouter()
	router.Post("/", l.ServeHTTP)
	return router
}

func (l Login) ServeHTTP(out http.ResponseWriter, in *http.Request) {
	var request struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}
	decodeRequestError := json.NewDecoder(in.Body).Decode(&request)
	if decodeRequestError != nil {
		status := http.StatusBadRequest
		http.Error(out, http.StatusText(status), status)
		return
	}

	token, authenticateError := l.IdentityProvider.Authenticate(in.Context(), request.Login, request.Password)
	if authenticateError != nil {
		status := http.StatusInternalServerError
		if errors.Is(authenticateError, idp.ErrBadCredentials) {
			status = http.StatusUnauthorized
		}
		http.Error(out, http.StatusText(status), status)
		return
	}
	out.Header().Set("Authorization", string(token))
	out.WriteHeader(http.StatusOK)
}
