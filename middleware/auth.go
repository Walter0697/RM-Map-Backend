package middleware

import (
	"context"
	"errors"
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
			user, err := service.ValidateToken(tokenStr)
			if err != nil {
				var unavailable *service.AuthStateUnavailableError
				if errors.As(err, &unavailable) {
					http.Error(w, "auth state unavailable", http.StatusServiceUnavailable)
					return
				}
				var tokenValidation *service.TokenValidationError
				if errors.As(err, &tokenValidation) {
					w.Header().Set("X-RM-Auth-Reason", tokenValidation.Reason)
					http.Error(w, tokenValidation.Error(), http.StatusUnauthorized)
					return
				}
				http.Error(w, "invalid token: unknown", http.StatusUnauthorized)
				return
			}
			if user == nil {
				w.Header().Set("X-RM-Auth-Reason", "missing_user_context")
				http.Error(w, "invalid token: missing_user_context", http.StatusUnauthorized)
				return
			}

			if !user.IsActivated {
				w.Header().Set("X-RM-Auth-Reason", "user_inactive")
				http.Error(w, "invalid token: user_inactive", http.StatusUnauthorized)
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
