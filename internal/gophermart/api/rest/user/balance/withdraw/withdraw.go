package withdraw

import (
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/kerelape/gophermart/internal/gophermart/api/rest/authorization"
	"github.com/kerelape/gophermart/internal/gophermart/idp"
	"net/http"
)

type Withdraw struct {
}

// New creates a new Withdraw.
func New() Withdraw {
	return Withdraw{}
}

func (w Withdraw) Route() http.Handler {
	router := chi.NewRouter()
	router.Post("/", w.ServeHTTP)
	return router
}

func (w Withdraw) ServeHTTP(out http.ResponseWriter, in *http.Request) {
	user := authorization.User(in)

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
