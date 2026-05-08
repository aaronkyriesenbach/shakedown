package recordings

import (
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"shakedown/internal/config"
)

type stubRouteHandler struct{}

func (s stubRouteHandler) Routes(_ chi.Router) {}

type stubTagHandler struct{}

func (s stubTagHandler) Routes(_ chi.Router)             {}
func (s stubTagHandler) RecordingTagRoutes(_ chi.Router) {}

func TestNewHandler(t *testing.T) {
	cfg := &config.Config{StorageRoot: t.TempDir()}
	h := NewHandler(nil, cfg, zap.NewNop())
	if h == nil {
		t.Fatal("expected non-nil handler from NewHandler")
	}
}

func TestRoutesRegistration(t *testing.T) {
	cfg := &config.Config{StorageRoot: t.TempDir()}
	h := NewHandler(nil, cfg, zap.NewNop())
	r := chi.NewRouter()
	requireAuth := func(next http.Handler) http.Handler { return next }

	h.Routes(r, requireAuth, stubRouteHandler{}, stubRouteHandler{}, stubTagHandler{})
}

func TestWaveformData_NotReady(t *testing.T) {
	t.Skip("requires RecordingRepository interface for mock injection — Repository.GetByID uses *pgxpool.Pool which cannot be mocked without an interface; refactor Handler.svc to accept a repository interface")
}

func TestWaveformData_Ready(t *testing.T) {
	t.Skip("requires RecordingRepository interface for mock injection — same as TestWaveformData_NotReady")
}

func TestStreamRecording_NotReady(t *testing.T) {
	t.Skip("requires RecordingRepository interface for mock injection — same as TestWaveformData_NotReady")
}
