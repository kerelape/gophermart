package orders

import (
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/kerelape/gophermart/internal/gophermart/api/rest/authorization"
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
	router.Use(authorization.Authorization(o.IdentityProvider))
	router.Post("/", o.upload)
	router.Get("/", o.list)
	return router
}

func (o Orders) upload(out http.ResponseWriter, in *http.Request) {
	user := authorization.User(in)

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

	user := authorization.User(in)

	orders, ordersError := user.Orders(in.Context())
	if ordersError != nil {
		log.Printf("failed to get orders: %v", ordersError)
		status := http.StatusInternalServerError
		http.Error(out, http.StatusText(status), status)
		return
	}
	if len(orders) == 0 {
		status := http.StatusNoContent
		http.Error(out, http.StatusText(status), status)
		return
	}

	response := make([]any, 0)
	for _, o := range orders {
		order := map[string]any{
			"number":     o.ID,
			"created_at": o.Time.Format(time.RFC3339),
			"status":     string(o.Status),
		}
		if o.Accrual > 0 {
			order["accrual"] = o.Accrual
		}
		response = append(response, order)
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
