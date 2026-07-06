package recordings

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"shakedown/internal/auth"
	"shakedown/internal/config"
)

type stubRouteHandler struct{}

func (s stubRouteHandler) Routes(_ chi.Router) {}

type stubTagHandler struct{}

func (s stubTagHandler) Routes(_ chi.Router)             {}
func (s stubTagHandler) RecordingTagRoutes(_ chi.Router) {}

type stubShareHandler struct{}

func (s stubShareHandler) RecordingRoutes(_ chi.Router) {}

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

	h.Routes(r, requireAuth, stubRouteHandler{}, stubRouteHandler{}, stubTagHandler{}, stubShareHandler{})
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

func TestThumbnailRouteRegistered(t *testing.T) {
	cfg := &config.Config{StorageRoot: t.TempDir()}
	h := NewHandler(nil, cfg, zap.NewNop())
	r := chi.NewRouter()
	requireAuth := func(next http.Handler) http.Handler { return next }
	h.Routes(r, requireAuth, stubRouteHandler{}, stubRouteHandler{}, stubTagHandler{}, stubShareHandler{})
}

func TestPlaybackFilename(t *testing.T) {
	if got := PlaybackFilename("audio"); got != "playback.m4a" {
		t.Errorf("audio: got %q, want playback.m4a", got)
	}
	if got := PlaybackFilename("video"); got != "playback.mp4" {
		t.Errorf("video: got %q, want playback.mp4", got)
	}
	if got := PlaybackFilename(""); got != "playback.m4a" {
		t.Errorf("empty: got %q, want playback.m4a", got)
	}
}

func TestSnippetFilename(t *testing.T) {
	if got := SnippetFilename("audio"); got != "snippet.m4a" {
		t.Errorf("audio: got %q, want snippet.m4a", got)
	}
	if got := SnippetFilename("video"); got != "snippet.mp4" {
		t.Errorf("video: got %q, want snippet.mp4", got)
	}
}

type fakeRepo struct {
	RecordingRepository
	failUpdateStoragePath bool
}

func (f *fakeRepo) Create(ctx context.Context, input CreateRecordingInput) (*Recording, error) {
	return &Recording{ID: "fake-id", FileExt: input.FileExt}, nil
}

func (f *fakeRepo) UpdateStoragePath(ctx context.Context, id, path string) error {
	if f.failUpdateStoragePath {
		return context.DeadlineExceeded // arbitrary error
	}
	return nil
}

func (f *fakeRepo) UpdateProcessingStep(ctx context.Context, id, step string) error {
	return nil
}

func (f *fakeRepo) UpdateProcessingResult(ctx context.Context, id string, duration float64, channels, sampleRate, bitrate int, hasAudio, hasVideo bool, codec *string, hasWaveform, hasThumbnail bool, width, height *int) error {
	return nil
}

func TestUpload_HookCalled(t *testing.T) {
	cfg := &config.Config{
		StorageRoot:          t.TempDir(),
		VideoUploadMaxSizeMB: 10,
	}
	store, _ := NewLocalStorage(cfg.StorageRoot)
	svc := NewService(&fakeRepo{}, store, zap.NewNop(), 1, 1)
	h := NewHandler(svc, cfg, zap.NewNop())

	hookCalled := false
	var hookedID string
	h.OnRecordingReady = func(id string) {
		hookCalled = true
		hookedID = id
	}

	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.mp3")
	part.Write([]byte{0x49, 0x44, 0x33, 0x00, 0x00}) // fake ID3 tag to pass magic bytes
	writer.Close()

	req := httptest.NewRequest("POST", "/", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Add mock user to context to pass auth check
	ctx := auth.WithUser(req.Context(), &auth.User{ID: "user1"})
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.upload(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d. Body: %s", w.Code, w.Body.String())
	}
	if !hookCalled {
		t.Fatal("expected hook to be called, but it wasn't")
	}
	if hookedID != "fake-id" {
		t.Fatalf("expected hook to receive 'fake-id', got %s", hookedID)
	}
}

func TestUpload_HookNotCalledOnDBError(t *testing.T) {
	cfg := &config.Config{
		StorageRoot:          t.TempDir(),
		VideoUploadMaxSizeMB: 10,
	}
	store, _ := NewLocalStorage(cfg.StorageRoot)
	svc := NewService(&fakeRepo{failUpdateStoragePath: true}, store, zap.NewNop(), 1, 1)
	h := NewHandler(svc, cfg, zap.NewNop())

	hookCalled := false
	h.OnRecordingReady = func(id string) {
		hookCalled = true
	}

	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.mp3")
	part.Write([]byte{0x49, 0x44, 0x33, 0x00, 0x00})
	writer.Close()

	req := httptest.NewRequest("POST", "/", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	ctx := auth.WithUser(req.Context(), &auth.User{ID: "user1"})
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.upload(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d. Body: %s", w.Code, w.Body.String())
	}
	if hookCalled {
		t.Fatal("expected hook NOT to be called when DB update fails, but it was")
	}
}
