package balance

import (
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"github.com/kerelape/gophermart/internal/gophermart/api/rest/authorization"
	"github.com/kerelape/gophermart/internal/gophermart/api/rest/user/balance/withdraw"
	"net/http"
)

type Balance struct {
	withdraw withdraw.Withdraw
}

// New creates a new Balance.
func New() Balance {
	return Balance{
		withdraw: withdraw.New(),
	}
}

func (b Balance) Route() http.Handler {
	router := chi.NewRouter()
	router.Mount("/withdraw", b.withdraw.Route())
	router.Get("/", b.ServeHTTP)
	return router
}

func (b Balance) ServeHTTP(out http.ResponseWriter, in *http.Request) {
	user := authorization.User(in)

	balance, balanceError := user.Balance(in.Context())
	if balanceError != nil {
		status := http.StatusInternalServerError
		http.Error(out, http.StatusText(status), status)
		return
	}

	response := map[string]any{
		"current":   balance.Current,
		"withdrawn": balance.Withdrawn,
	}

	responseBody, marshalResponseBodyError := json.Marshal(response)
	if marshalResponseBodyError != nil {
		status := http.StatusInternalServerError
		http.Error(out, http.StatusText(status), status)
		return
	}

	if _, err := out.Write(responseBody); err != nil {
		status := http.StatusInternalServerError
		http.Error(out, http.StatusText(status), status)
		return
	}

	out.Header().Set("Content-Type", "application/json")
	out.WriteHeader(http.StatusOK)
}
