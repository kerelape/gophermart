package idp

import "context"

type Identity interface {
	User

	// ComparePassword compares the provided password by the password of this identity.
	ComparePassword(ctx context.Context, password string) (bool, error)
}
