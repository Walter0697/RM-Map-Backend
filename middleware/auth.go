package middleware

import (
	"context"
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/service"
	"net/http"
)

var userCtxKey = &contextKey{"user"}

type contextKey struct {
	name string
}

func Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")

			if header == "" {
				next.ServeHTTP(w, r)
				return
			}

			// validate jwt token
			tokenStr := header
			user := service.ValidateToken(tokenStr)
			if user == nil {
				http.Error(w, "Invalid token", http.StatusUnauthorized)
				return
			}

			if !user.IsActivated {
				http.Error(w, "Invalid token", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), userCtxKey, user)

			r = r.WithContext(ctx)
			next.ServeHTTP(w, r)
		})
	}
}

func ForContext(ctx context.Context) *dbmodel.User {
	raw, _ := ctx.Value(userCtxKey).(*dbmodel.User)
	return raw
}
