package withdrawals

import (
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"github.com/kerelape/gophermart/internal/gophermart/api/rest/authorization"
	"net/http"
	"time"
)

type Withdrawals struct {
}

// New creates a new Withdrawals.
func New() Withdrawals {
	return Withdrawals{}
}

func (w Withdrawals) Route() http.Handler {
	router := chi.NewRouter()
	router.Get("/", w.ServeHTTP)
	return router
}

func (w Withdrawals) ServeHTTP(out http.ResponseWriter, in *http.Request) {
	out.Header().Set("Content-Type", "application/json")

	user := authorization.User(in)

	withdrawals, withdrawalsError := user.Withdrawals(in.Context())
	if withdrawalsError != nil {
		status := http.StatusInternalServerError
		http.Error(out, http.StatusText(status), status)
		return
	}
	if len(withdrawals) == 0 {
		status := http.StatusNoContent
		http.Error(out, http.StatusText(status), status)
		return
	}

	response := make([]map[string]any, len(withdrawals))
	for i, withdrawal := range withdrawals {
		response[i] = map[string]any{
			"order":        withdrawal.Order,
			"sum":          withdrawal.Sum,
			"processed_at": withdrawal.Time.Format(time.RFC3339),
		}
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

	out.WriteHeader(http.StatusOK)
}
