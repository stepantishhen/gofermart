// Package middleware provides the HTTP middleware used by the loyalty server:
// authentication, gzip compression, and request logging.
package middleware

import (
	"context"
	"net/http"
	"strings"
)

// TokenParser validates a token string and returns the user ID it carries.
type TokenParser interface {
	Parse(token string) (int64, error)
}

type ctxKey int

const userIDKey ctxKey = iota

// AuthCookieName is the cookie the server also accepts (and sets) for the token.
const AuthCookieName = "token"

// Auth returns middleware that rejects requests without a valid token (401) and
// stores the authenticated user ID in the request context.
func Auth(parser TokenParser) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := bearerToken(r)
			if token == "" {
				if c, err := r.Cookie(AuthCookieName); err == nil {
					token = c.Value
				}
			}
			userID, err := parser.Parse(token)
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), userIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserID returns the authenticated user ID stored by Auth.
func UserID(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(userIDKey).(int64)
	return id, ok
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if after, ok := strings.CutPrefix(h, "Bearer "); ok {
		return strings.TrimSpace(after)
	}
	return ""
}
