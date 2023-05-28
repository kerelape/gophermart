package balance

import (
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/kerelape/gophermart/internal/gophermart/api/rest/user/balance/withdraw"
	"github.com/kerelape/gophermart/internal/gophermart/idp"
	"net/http"
)

type Balance struct {
	withdraw withdraw.Withdraw

	IdentityProvider idp.IdentityProvider
}

// New creates a new Balance.
func New(identityProvider idp.IdentityProvider) Balance {
	return Balance{
		withdraw: withdraw.New(identityProvider),

		IdentityProvider: identityProvider,
	}
}

func (b Balance) Route() http.Handler {
	router := chi.NewRouter()
	router.Get("/", b.ServeHTTP)
	return router
}

func (b Balance) ServeHTTP(out http.ResponseWriter, in *http.Request) {
	token := in.Header.Get("Authorization")
	user, userError := b.IdentityProvider.User(in.Context(), idp.Token(token))
	if userError != nil {
		status := http.StatusInternalServerError
		if errors.Is(userError, idp.ErrBadCredentials) {
			status = http.StatusUnauthorized
		}
		http.Error(out, http.StatusText(status), status)
		return
	}

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
	out.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(out).Encode(response); err != nil {
		panic(err)
	}
}
