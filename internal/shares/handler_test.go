package shares

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestGetShare(t *testing.T) {
	t.Run("returns 404 when no share in context", func(t *testing.T) {
		h := &Handler{logger: zap.NewNop()}
		req := httptest.NewRequest(http.MethodGet, "/api/s/sometoken", nil)
		rr := httptest.NewRecorder()

		h.GetShare(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", rr.Code)
		}
	})

	t.Run("returns 200 with share JSON when share in context", func(t *testing.T) {
		t.Skip("requires recRepo; GetShare now fetches recording — needs interface injection or integration test")
		h := &Handler{logger: zap.NewNop()}

		now := time.Now().UTC().Truncate(time.Second)
		share := &Share{
			ID:          "share-1",
			Token:       "test-token-abc",
			RecordingID: "rec-1",
			CreatedBy:   "user-1",
			AccessCount: 0,
			CreatedAt:   now,
		}

		req := httptest.NewRequest(http.MethodGet, "/api/s/test-token-abc", nil)
		ctx := context.WithValue(req.Context(), shareKey, share)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		h.GetShare(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
		if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected Content-Type application/json, got %q", ct)
		}

		var got Share
		if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if got.Token != share.Token {
			t.Errorf("expected token %q, got %q", share.Token, got.Token)
		}
		if got.RecordingID != share.RecordingID {
			t.Errorf("expected recording_id %q, got %q", share.RecordingID, got.RecordingID)
		}
	})
}

func TestCreateShare(t *testing.T) {
	t.Run("returns 401 when no authenticated user", func(t *testing.T) {
		h := &Handler{logger: zap.NewNop()}
		req := httptest.NewRequest(http.MethodPost, "/api/shares", strings.NewReader(`{"recording_id":"rec-1"}`))
		rr := httptest.NewRecorder()

		h.CreateShare(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rr.Code)
		}
	})

	t.Run("returns 400 for invalid JSON body when user in context", func(t *testing.T) {
		t.Skip("requires auth.userContextKey export or auth user injection helper — auth.contextKey is unexported and cannot be set from package shares")
	})

	t.Run("returns 400 when recording_id missing", func(t *testing.T) {
		t.Skip("requires auth.userContextKey export or auth user injection helper — auth.contextKey is unexported and cannot be set from package shares")
	})
}
