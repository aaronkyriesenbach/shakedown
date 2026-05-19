package auth

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type contextKey string

const userContextKey contextKey = "user"

// RequireAuth validates the session cookie and injects the user into context.
// Returns 401 if no valid session is found.
func RequireAuth(db *pgxpool.Pool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("shakedown_session")
			if err != nil {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			user, err := GetSession(r.Context(), db, cookie.Value)
			if err != nil || user == nil {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), userContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// DevAuth injects a synthetic local dev user into every request, bypassing
// session validation. Only used when DISABLE_AUTH=true.
// It upserts the dev user into the database so that foreign key constraints
// (e.g. recordings.uploaded_by) are satisfied.
func DevAuth(db *pgxpool.Pool, logger *zap.Logger) func(http.Handler) http.Handler {
	devUser := &User{
		ID:          "00000000-0000-0000-0000-000000000000",
		OIDCSub:     "dev",
		Email:       "dev@localhost",
		DisplayName: "Local Dev",
		Role:        "admin",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if db != nil {
		_, err := db.Exec(context.Background(),
			`INSERT INTO users (id, oidc_sub, email, display_name, role)
			 VALUES ($1, $2, $3, $4, $5)
			 ON CONFLICT (id) DO NOTHING`,
			devUser.ID, devUser.OIDCSub, devUser.Email, devUser.DisplayName, devUser.Role,
		)
		if err != nil {
			logger.Error("failed to upsert dev user", zap.Error(err))
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), userContextKey, devUser)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAdmin checks that the authenticated user has admin role.
// Must be used after RequireAuth.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r.Context())
		if user == nil || user.Role != "admin" {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// UserFromContext retrieves the User from the request context.
// Returns nil if no user is present.
func UserFromContext(ctx context.Context) *User {
	user, _ := ctx.Value(userContextKey).(*User)
	return user
}
