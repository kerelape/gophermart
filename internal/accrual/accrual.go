package accrual

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

var (
	ErrUnknownOrder    = errors.New("unknown order")
	ErrTooManyRequests = errors.New("too many requests")
)

type Accrual struct {
	Address string
	Client  *http.Client
}

// New creates a new Accrual.
func New(address string, client *http.Client) Accrual {
	return Accrual{
		Address: address,
		Client:  client,
	}
}

func (a Accrual) OrderInfo(ctx context.Context, order string) (OrderInfo, error) {
	out, outError := http.NewRequestWithContext(ctx, http.MethodGet, a.Address+"/api/orders/"+order, strings.NewReader(""))
	if outError != nil {
		return OrderInfo{}, outError
	}

	in, doError := a.Client.Do(out)
	if doError != nil {
		return OrderInfo{}, doError
	}
	defer in.Body.Close()
	if in.StatusCode != http.StatusOK {
		switch in.StatusCode {
		case http.StatusTooManyRequests:
			return OrderInfo{}, ErrTooManyRequests
		case http.StatusNoContent:
			return OrderInfo{}, ErrUnknownOrder
		default:
			panic(fmt.Errorf("unknown response code %d", in.StatusCode))
		}
	}

	var response struct {
		Order   string      `json:"order"`
		Status  OrderStatus `json:"status"`
		Accrual float64     `json:"accrual,omitempty"`
	}
	if err := json.NewDecoder(in.Body).Decode(&response); err != nil {
		return OrderInfo{}, err
	}

	return OrderInfo{response.Order, response.Status, response.Accrual}, nil
}
