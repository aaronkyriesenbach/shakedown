package shares

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type shareContextKey string

const shareKey shareContextKey = "share"

func TokenMiddleware(db *pgxpool.Pool) func(http.Handler) http.Handler {
	repo := NewRepository(db)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := chi.URLParam(r, "token")
			share, err := repo.GetByToken(r.Context(), token)
			if err != nil || share == nil {
				http.Error(w, `{"error":"share not found or expired"}`, http.StatusNotFound)
				return
			}
			_ = repo.IncrementAccess(r.Context(), share.ID)
			ctx := context.WithValue(r.Context(), shareKey, share)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func ShareFromContext(ctx context.Context) *Share {
	s, _ := ctx.Value(shareKey).(*Share)
	return s
}
