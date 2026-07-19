package middleware

import (
	"context"

	"livestock/internal/models"
)

type contextKey int

const (
	userContextKey contextKey = iota
	sessionContextKey
)

// UserFromContext returns the authenticated user attached by Authenticate,
// or ok=false if the request reached this point without passing through it.
func UserFromContext(ctx context.Context) (*models.User, bool) {
	u, ok := ctx.Value(userContextKey).(*models.User)
	return u, ok
}

// SessionFromContext returns the validated session attached by Authenticate.
func SessionFromContext(ctx context.Context) (*models.Session, bool) {
	s, ok := ctx.Value(sessionContextKey).(*models.Session)
	return s, ok
}

func withUser(ctx context.Context, u *models.User) context.Context {
	return context.WithValue(ctx, userContextKey, u)
}

func withSession(ctx context.Context, s *models.Session) context.Context {
	return context.WithValue(ctx, sessionContextKey, s)
}
