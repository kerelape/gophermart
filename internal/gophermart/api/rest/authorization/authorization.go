package authorization

import (
	"context"
	"errors"
	"github.com/kerelape/gophermart/internal/gophermart/idp"
	"net/http"
)

type contextKey string

const ContextKeyUser = contextKey("authorization.user")

func Authorization(identityProvider idp.IdentityProvider) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(out http.ResponseWriter, in *http.Request) {
			token := in.Header.Get("Authorization")
			user, err := identityProvider.User(in.Context(), idp.Token(token))
			if err != nil {
				status := http.StatusInternalServerError
				if errors.Is(err, idp.ErrBadCredentials) {
					status = http.StatusUnauthorized
				}
				http.Error(out, http.StatusText(status), status)
				return
			}
			next.ServeHTTP(out, in.WithContext(context.WithValue(in.Context(), ContextKeyUser, user)))
		})
	}
}

func User(in *http.Request) idp.User {
	user, ok := in.Context().Value(ContextKeyUser).(idp.User)
	if !ok {
		panic("user not defined (authorization middleware is not used)")
	}
	return user
}
