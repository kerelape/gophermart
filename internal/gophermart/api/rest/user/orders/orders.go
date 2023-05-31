package orders

import (
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/kerelape/gophermart/internal/gophermart/idp"
	"io"
	"log"
	"net/http"
	"time"
)

type Orders struct {
	IdentityProvider idp.IdentityProvider
}

// New creates a new Orders.
func New(identityProvider idp.IdentityProvider) Orders {
	return Orders{
		IdentityProvider: identityProvider,
	}
}

func (o Orders) Route() http.Handler {
	router := chi.NewRouter()
	router.Post("/", o.upload)
	router.Get("/", o.list)
	return router
}

func (o Orders) upload(out http.ResponseWriter, in *http.Request) {
	token := in.Header.Get("Authorization")
	user, userError := o.IdentityProvider.User(in.Context(), idp.Token(token))
	if userError != nil {
		log.Printf("failed to get user: %v", userError)
		status := http.StatusInternalServerError
		if errors.Is(userError, idp.ErrBadCredentials) {
			status = http.StatusUnauthorized
		}
		http.Error(out, http.StatusText(status), status)
		return
	}

	order, readOrderError := io.ReadAll(in.Body)
	if readOrderError != nil {
		status := http.StatusBadRequest
		http.Error(out, http.StatusText(status), status)
		return
	}

	addOrderError := user.AddOrder(in.Context(), string(order))
	if addOrderError != nil {
		log.Printf("failed to add order: %v", addOrderError)
		if errors.Is(addOrderError, idp.ErrOrderDuplicate) {
			out.WriteHeader(http.StatusOK)
			return
		}
		status := http.StatusInternalServerError
		if errors.Is(addOrderError, idp.ErrOrderInvalid) {
			status = http.StatusUnprocessableEntity
		}
		if errors.Is(addOrderError, idp.ErrOrderUnowned) {
			status = http.StatusConflict
		}
		http.Error(out, http.StatusText(status), status)
		return
	}
	out.WriteHeader(http.StatusAccepted)
}

func (o Orders) list(out http.ResponseWriter, in *http.Request) {
	out.Header().Set("Content-Type", "application/json")

	token := in.Header.Get("Authorization")
	user, userError := o.IdentityProvider.User(in.Context(), idp.Token(token))
	if userError != nil {
		status := http.StatusInternalServerError
		if errors.Is(userError, idp.ErrBadCredentials) {
			status = http.StatusUnauthorized
		}
		http.Error(out, http.StatusText(status), status)
		return
	}

	orders, ordersError := user.Orders(in.Context())
	if ordersError != nil {
		status := http.StatusInternalServerError
		http.Error(out, http.StatusText(status), status)
		return
	}
	if len(orders) == 0 {
		status := http.StatusNoContent
		http.Error(out, http.StatusText(status), status)
		return
	}

	response := make([]map[string]any, 0, len(orders))
	for i, order := range response {
		order["number"] = orders[i].ID
		order["created_at"] = orders[i].Time.Format(time.RFC3339)
		order["status"] = string(orders[i].Status)
		if orders[i].Accrual > 0 {
			order["accrual"] = orders[i].Accrual
		}
	}
	out.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(out).Encode(response); err != nil {
		panic(err)
	}
}
