package withdrawals

import (
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/kerelape/gophermart/internal/gophermart/idp"
	"net/http"
	"time"
)

type Withdrawals struct {
	IdentityProvider idp.IdentityProvider
}

// New creates a new Withdrawals.
func New(identityProvider idp.IdentityProvider) Withdrawals {
	return Withdrawals{
		IdentityProvider: identityProvider,
	}
}

func (w Withdrawals) Route() http.Handler {
	router := chi.NewRouter()
	router.Get("/", w.ServeHTTP)
	return router
}

func (w Withdrawals) ServeHTTP(out http.ResponseWriter, in *http.Request) {
	out.Header().Set("Content-Type", "application/json")

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
	out.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(out).Encode(response); err != nil {
		panic(err)
	}
}
