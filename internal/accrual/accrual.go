package accrual

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

var ErrUnknownOrder = errors.New("unknown order")

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
	out, outError := http.NewRequestWithContext(ctx, http.MethodGet, a.Address+"/api/orders/"+order, nil)
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
			retryAfter, err := strconv.Atoi(in.Header.Get("Retry-After"))
			if err != nil {
				return OrderInfo{}, err
			}
			return OrderInfo{}, TooManyRequestsError{time.Second * time.Duration(retryAfter)}
		case http.StatusNoContent:
			return OrderInfo{}, ErrUnknownOrder
		default:
			return OrderInfo{}, fmt.Errorf("unknown response code %d", in.StatusCode)
		}
	}

	info := OrderInfo{}
	if err := json.NewDecoder(in.Body).Decode(&info); err != nil {
		return OrderInfo{}, err
	}

	return info, nil
}
