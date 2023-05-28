package idp

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrOrderInvalidFormat is returned when an order ID is in wrong format.
	ErrOrderInvalidFormat = errors.New("invalid order format")

	// ErrOrderDuplicate is returned when the order has already been added to this user.
	ErrOrderDuplicate = errors.New("duplicate order")

	// ErrOrderUnowned is returned when the user attempted to add an order that another user already has.
	ErrOrderUnowned = errors.New("unowned order")
)

// User represents a Gophermart client.
type User interface {
	// AddOrder adds an order to the user.
	AddOrder(ctx context.Context, id string) error

	Orders(ctx context.Context) ([]Order, error)

	// Balance returns current balance status.
	Balance(ctx context.Context) (Balance, error)
}

type Order struct {
	ID      string
	Status  OrderStatus
	Accrual int // -1 means no Accrual.
	Time    time.Time
}

type OrderStatus string

var (
	OrderStatusNew        OrderStatus = "NEW"
	OrderStatusProcessing             = "PROCESSING"
	OrderStatusInvalid                = "INVALID"
	OrderStatusProcessed              = "PROCESSED"
)
