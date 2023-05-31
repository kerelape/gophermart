package idp

import "context"

type IdentityDatabase interface {
	// Create creates a new identity in the database.
	Create(ctx context.Context, username, password string) error

	// Identity returns the identity by the provided username.
	Identity(username string) Identity
}
