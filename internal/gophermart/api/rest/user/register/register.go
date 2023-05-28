package register

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/kerelape/gophermart/internal/gophermart/idp"
)

type Register struct {
	idp idp.IdentityProvider
}

// New creates a new Register.
func New(idp idp.IdentityProvider) Register {
	return Register{
		idp: idp,
	}
}

func (r Register) Route() http.Handler {
	router := chi.NewRouter()
	router.Post("/", r.ServeHTTP)
	return router
}

func (r Register) ServeHTTP(out http.ResponseWriter, in *http.Request) {
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

	// Register the user.
	registerError := r.idp.Register(in.Context(), request.Login, request.Password)
	if registerError != nil {
		status := http.StatusInternalServerError
		if errors.Is(registerError, idp.ErrDuplicateUsername) {
			status = http.StatusConflict
		}
		http.Error(out, http.StatusText(status), status)
		return
	}

	// Authenticate the user.
	token, authenticateError := r.idp.Authenticate(in.Context(), request.Login, request.Password)
	if authenticateError != nil {
		status := http.StatusInternalServerError
		http.Error(out, http.StatusText(status), status)
		return
	}
	out.Header().Add("Authorization", string(token))
	out.WriteHeader(http.StatusOK)
}
