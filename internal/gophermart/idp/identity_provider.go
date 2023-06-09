package idp

import (
	"context"
	"errors"
)

type Token string

var (
	// ErrDuplicateUsername is returned when a user failed to register due to
	// their username being a duplicate.
	ErrDuplicateUsername = errors.New("duplicate username")

	// ErrBadCredentials is returned when provided credentials are wrong (username/password or token).
	ErrBadCredentials = errors.New("bad credentials")
)

type IdentityProvider interface {

	// Register registers a new user.
	Register(ctx context.Context, username, password string) error

	// Authenticate authenticates the user.
	Authenticate(ctx context.Context, username, password string) (Token, error)

	// User returns the User associated with the token.
	User(ctx context.Context, token Token) (User, error)
}
