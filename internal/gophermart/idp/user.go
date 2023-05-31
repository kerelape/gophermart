package idp

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrOrderInvalid is returned when an order Order is in wrong format.
	ErrOrderInvalid = errors.New("invalid order format")

	// ErrOrderDuplicate is returned when the order has already been added to this user.
	ErrOrderDuplicate = errors.New("duplicate order")

	// ErrOrderUnowned is returned when the user attempted to add an order that another user already has.
	ErrOrderUnowned = errors.New("unowned order")

	ErrBalanceTooLow = errors.New("balance too low")
)

// User represents a Gophermart client.
type User interface {
	// AddOrder adds an order to the user.
	AddOrder(ctx context.Context, id string) error

	Orders(ctx context.Context) ([]Order, error)

	// Balance returns current balance status.
	Balance(ctx context.Context) (Balance, error)

	// Withdraw withdraws amount towards order.
	Withdraw(ctx context.Context, order string, amount float64) error

	// Withdrawals returns withdrawals history.
	Withdrawals(ctx context.Context) ([]Withdrawal, error)
}

type Withdrawal struct {
	Order string
	Sum   float64
	Time  time.Time
}

type Order struct {
	ID      string
	Status  OrderStatus
	Accrual float64
	Time    time.Time
}

type OrderStatus string

func (o OrderStatus) IsFinal() bool {
	switch o {
	case OrderStatusNew:
		return false
	case OrderStatusProcessing:
		return false
	case OrderStatusInvalid:
		return true
	case OrderStatusProcessed:
		return true
	}
	panic("unsupported order status")
}

var (
	OrderStatusNew        = OrderStatus("NEW")
	OrderStatusInvalid    = OrderStatus("INVALID")
	OrderStatusProcessing = OrderStatus("PROCESSING")
	OrderStatusProcessed  = OrderStatus("PROCESSED")
)
