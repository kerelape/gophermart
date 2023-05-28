package withdraw

import (
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/kerelape/gophermart/internal/gophermart/idp"
	"net/http"
)

type Withdraw struct {
	IdentityProvider idp.IdentityProvider
}

// New creates a new Withdraw.
func New(identityProvider idp.IdentityProvider) Withdraw {
	return Withdraw{
		IdentityProvider: identityProvider,
	}
}

func (w Withdraw) Route() http.Handler {
	router := chi.NewRouter()
	router.Post("/", w.ServeHTTP)
	return router
}

func (w Withdraw) ServeHTTP(out http.ResponseWriter, in *http.Request) {
	token := in.Header.Get("Authorization")
	user, userError := w.IdentityProvider.User(in.Context(), idp.Token(token))
	if userError != nil {
		status := http.StatusInternalServerError
		if errors.Is(userError, idp.ErrBadCredentials) {
			status = http.StatusUnauthorized
		}
		http.Error(out, http.StatusText(status), status)
		return
	}

	var request struct {
		Order string  `json:"order"`
		Sum   float64 `json:"sum"`
	}
	decodeRequestError := json.NewDecoder(in.Body).Decode(&request)
	if decodeRequestError != nil {
		status := http.StatusBadRequest
		http.Error(out, http.StatusText(status), status)
		return
	}

	withdrawError := user.Withdraw(in.Context(), request.Order, request.Sum)
	if withdrawError != nil {
		status := http.StatusInternalServerError
		if errors.Is(withdrawError, idp.ErrBalanceTooLow) {
			status = http.StatusPaymentRequired
		}
		if errors.Is(withdrawError, idp.ErrOrderInvalid) {
			status = http.StatusUnprocessableEntity
		}
		http.Error(out, http.StatusText(status), status)
		return
	}

	out.WriteHeader(http.StatusOK)
}
