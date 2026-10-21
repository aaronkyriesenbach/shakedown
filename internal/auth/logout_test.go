package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

func TestEndSessionURL(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		want     string
	}{
		{name: "provider without end_session_endpoint", endpoint: "", want: ""},
		{
			name:     "adds client_id",
			endpoint: "https://idp.example/logout",
			want:     "https://idp.example/logout?client_id=shakedown",
		},
		{
			name:     "keeps an existing query",
			endpoint: "https://idp.example/logout?tenant=a",
			want:     "https://idp.example/logout?client_id=shakedown&tenant=a",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Provider{
				endSessionEndpoint: tt.endpoint,
				oauthConfig:        oauth2.Config{ClientID: "shakedown"},
			}

			if got := p.EndSessionURL(); got != tt.want {
				t.Errorf("EndSessionURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLogoutHandler(t *testing.T) {
	t.Run("returns 204 when the provider has no end_session_endpoint", func(t *testing.T) {
		h := &Handler{provider: &Provider{}, logger: zap.NewNop()}
		rr := httptest.NewRecorder()

		h.logout(rr, httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil))

		if rr.Code != http.StatusNoContent {
			t.Errorf("expected 204, got %d", rr.Code)
		}
		if rr.Body.Len() != 0 {
			t.Errorf("expected empty body, got %q", rr.Body.String())
		}
	})

	t.Run("returns the end_session_url when the provider has one", func(t *testing.T) {
		h := &Handler{
			provider: &Provider{
				endSessionEndpoint: "https://idp.example/logout",
				oauthConfig:        oauth2.Config{ClientID: "shakedown"},
			},
			logger: zap.NewNop(),
		}
		rr := httptest.NewRecorder()

		h.logout(rr, httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil))

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rr.Code)
		}
		var body struct {
			EndSessionURL string `json:"end_session_url"`
		}
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode response body: %v", err)
		}
		if want := "https://idp.example/logout?client_id=shakedown"; body.EndSessionURL != want {
			t.Errorf("end_session_url = %q, want %q", body.EndSessionURL, want)
		}
	})

	t.Run("always clears the session cookie", func(t *testing.T) {
		h := &Handler{provider: &Provider{}, logger: zap.NewNop()}
		rr := httptest.NewRecorder()

		h.logout(rr, httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil))

		var cleared bool
		for _, c := range rr.Result().Cookies() {
			if c.Name == "shakedown_session" && c.MaxAge < 0 {
				cleared = true
			}
		}
		if !cleared {
			t.Error("expected shakedown_session to be cleared")
		}
	})
}
