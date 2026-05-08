package auth

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"shakedown/internal/config"
)

// Handler holds dependencies for auth HTTP handlers.
type Handler struct {
	db       *pgxpool.Pool
	provider *Provider
	cfg      *config.Config
	logger   *zap.Logger
}

// NewHandler creates an auth Handler.
func NewHandler(db *pgxpool.Pool, provider *Provider, cfg *config.Config, logger *zap.Logger) *Handler {
	return &Handler{db: db, provider: provider, cfg: cfg, logger: logger}
}

// Routes registers all auth routes on the given chi.Router.
// The /me endpoint is protected by RequireAuth.
func (h *Handler) Routes(r chi.Router) {
	r.Get("/login", h.login)
	r.Get("/callback", h.callback)
	r.Post("/logout", h.logout)
	r.With(RequireAuth(h.db)).Get("/me", h.me)
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	state, err := GenerateRandomString(16)
	if err != nil {
		h.logger.Error("failed to generate state", zap.Error(err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	nonce, err := GenerateRandomString(16)
	if err != nil {
		h.logger.Error("failed to generate nonce", zap.Error(err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "oidc_state",
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   600,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "oidc_nonce",
		Value:    nonce,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   600,
	})

	http.Redirect(w, r, h.provider.AuthCodeURL(state, nonce), http.StatusFound)
}

func (h *Handler) callback(w http.ResponseWriter, r *http.Request) {
	stateCookie, err := r.Cookie("oidc_state")
	if err != nil || stateCookie.Value != r.URL.Query().Get("state") {
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}

	nonceCookie, err := r.Cookie("oidc_nonce")
	if err != nil {
		http.Error(w, "missing nonce", http.StatusBadRequest)
		return
	}

	idToken, _, err := h.provider.Exchange(r.Context(), r.URL.Query().Get("code"), nonceCookie.Value)
	if err != nil {
		h.logger.Error("OIDC exchange failed", zap.Error(err))
		http.Error(w, "authentication failed", http.StatusUnauthorized)
		return
	}

	var claims struct {
		Sub     string `json:"sub"`
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}
	if err := idToken.Claims(&claims); err != nil {
		http.Error(w, "invalid token claims", http.StatusInternalServerError)
		return
	}

	var avatarURL *string
	if claims.Picture != "" {
		avatarURL = &claims.Picture
	}

	user, err := UpsertUser(r.Context(), h.db, claims.Sub, claims.Email, claims.Name, avatarURL)
	if err != nil {
		h.logger.Error("failed to upsert user", zap.Error(err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	session, err := CreateSession(r.Context(), h.db, user.ID)
	if err != nil {
		h.logger.Error("failed to create session", zap.Error(err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "shakedown_session",
		Value:    session.ID,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		Expires:  session.ExpiresAt,
	})

	for _, name := range []string{"oidc_state", "oidc_nonce"} {
		http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: "/", MaxAge: -1})
	}

	http.Redirect(w, r, "/", http.StatusFound)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("shakedown_session")
	if err == nil {
		_ = DeleteSession(r.Context(), h.db, cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:    "shakedown_session",
		Value:   "",
		Path:    "/",
		MaxAge:  -1,
		Expires: time.Unix(0, 0),
	})
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(user)
}
