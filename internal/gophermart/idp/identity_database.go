package idp

import "context"

type IdentityDatabase interface {
	// Create creates a new identity in the database.
	Create(ctx context.Context, username, password string) error

	// Identity returns the identity by the provided username.
	Identity(username string) Identity

	// UpdateOrder updates status and accrual of an order
	UpdateOrder(ctx context.Context, id string, newStatus OrderStatus, accrual float64) error

	// Orders returns orders.
	Orders(ctx context.Context, statuses ...OrderStatus) ([]Order, error)
}
