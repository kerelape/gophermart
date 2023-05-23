package user

import "net/http"

type User struct {
}

func New() User {
	return User{}
}

func (u User) Route() http.Handler {
	return nil // TODO
}
