package idp

import (
	"context"
	"github.com/golang-jwt/jwt/v5"
	"strings"
	"time"
)

type BearerIdentityProvider struct {
	database IdentityDatabase
	secret   []byte
}

// NewBearerIdentityProvider creates a new BearerIdentityProvider.
func NewBearerIdentityProvider(database IdentityDatabase, secret []byte) BearerIdentityProvider {
	return BearerIdentityProvider{
		database: database,
		secret:   secret,
	}
}

func (b BearerIdentityProvider) Register(ctx context.Context, username, password string) error {
	return b.database.Create(ctx, username, password)
}

func (b BearerIdentityProvider) Authenticate(ctx context.Context, username, password string) (Token, error) {
	authenticated, comparePasswordError := b.database.Identity(username).ComparePassword(ctx, password)
	if comparePasswordError != nil {
		return "", comparePasswordError
	}
	if !authenticated {
		return "", ErrBadCredentials
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"exp": time.Now().Add(time.Hour * 24).Unix(),
		"iss": "https://github.com/kerelape/gophermart",
		"sub": username,
	})
	signedToken, signTokenError := token.SignedString(b.secret)
	if signTokenError != nil {
		return "", signTokenError
	}
	return Token("Bearer " + signedToken), nil
}

func (b BearerIdentityProvider) User(_ context.Context, token Token) (User, error) {
	rawToken, hasPrefix := strings.CutPrefix(string(token), "Bearer ")
	if !hasPrefix {
		return nil, ErrBadCredentials
	}

	parsedToken, parseTokenError := jwt.Parse(
		rawToken,
		func(token *jwt.Token) (interface{}, error) {
			return b.secret, nil
		},
	)
	if parseTokenError != nil {
		return nil, ErrBadCredentials
	}

	exp, expError := parsedToken.Claims.GetExpirationTime()
	if expError != nil {
		return nil, ErrBadCredentials
	}
	if exp.Before(time.Now()) {
		return nil, ErrBadCredentials
	}

	sub, subError := parsedToken.Claims.GetSubject()
	if subError != nil {
		return nil, ErrBadCredentials
	}

	return b.database.Identity(sub), nil
}
