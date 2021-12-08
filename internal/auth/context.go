package auth

import (
	"context"
)

type User struct {
	Name string
}

type userKey struct{}

func WithUser(ctx context.Context, user User) context.Context {
	return context.WithValue(ctx, userKey{}, &user)
}

func UserFromContext(ctx context.Context) *User {
	if v, ok := ctx.Value(userKey{}).(*User); ok {
		return v
	}
	return nil
}
