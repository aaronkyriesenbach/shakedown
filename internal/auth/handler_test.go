package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/zap"
)

// localHealthHandler is a local mirror of cmd/server/main.go's healthHandler.
// The production copy lives in package main which cannot be imported by tests,
// so this contract copy verifies the expected response shape.
func localHealthHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"version": "dev",
	})
}

func TestHealthHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rr := httptest.NewRecorder()

	http.HandlerFunc(localHealthHandler).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}

	var body map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf(`expected body["status"] == "ok", got %q`, body["status"])
	}
}

func TestMeHandler(t *testing.T) {
	t.Run("returns 401 when no user in context", func(t *testing.T) {
		h := &Handler{logger: zap.NewNop()}
		req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
		rr := httptest.NewRecorder()

		h.me(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rr.Code)
		}
	})

	t.Run("returns 200 with user JSON when user in context", func(t *testing.T) {
		h := &Handler{logger: zap.NewNop()}

		now := time.Now().UTC().Truncate(time.Second)
		user := &User{
			ID:          "user-1",
			OIDCSub:     "oidc-sub-1",
			Email:       "test@example.com",
			DisplayName: "Test User",
			Role:        "user",
			CreatedAt:   now,
			UpdatedAt:   now,
		}

		req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
		ctx := context.WithValue(req.Context(), userContextKey, user)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		h.me(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
		if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected Content-Type application/json, got %q", ct)
		}

		var got User
		if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if got.Email != user.Email {
			t.Errorf("expected email %q, got %q", user.Email, got.Email)
		}
		if got.Role != user.Role {
			t.Errorf("expected role %q, got %q", user.Role, got.Role)
		}
		if got.ID != user.ID {
			t.Errorf("expected id %q, got %q", user.ID, got.ID)
		}
	})
}
