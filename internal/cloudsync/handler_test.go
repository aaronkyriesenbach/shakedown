package cloudsync

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"shakedown/internal/auth"
	"shakedown/internal/recordings"
)

// handlerFakeRemoteClient is a RemoteClient fake with per-test configurable
// RemoteExists/WriteRemoteConfig behavior (the shared fakeRemoteClient in
// service_test.go hardcodes RemoteExists=true/WriteRemoteConfig=nil).
type handlerFakeRemoteClient struct {
	remoteExistsFn func(ctx context.Context) (bool, error)
	writeConfigFn  func(ctx context.Context, block string) error
	lastBlock      string
}

func (f *handlerFakeRemoteClient) Version(ctx context.Context) (string, error) { return "", nil }

func (f *handlerFakeRemoteClient) RemoteExists(ctx context.Context) (bool, error) {
	if f.remoteExistsFn != nil {
		return f.remoteExistsFn(ctx)
	}
	return true, nil
}

func (f *handlerFakeRemoteClient) WriteRemoteConfig(ctx context.Context, block string) error {
	f.lastBlock = block
	if f.writeConfigFn != nil {
		return f.writeConfigFn(ctx, block)
	}
	return nil
}

func (f *handlerFakeRemoteClient) Copy(ctx context.Context, localAbsPath, remotePath string) error {
	return nil
}

func (f *handlerFakeRemoteClient) StatSize(ctx context.Context, remotePath string) (int64, bool, error) {
	return 0, false, nil
}

func adminRequest(method, path string, body *strings.Reader) *http.Request {
	var req *http.Request
	if body == nil {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, body)
	}
	ctx := auth.WithUser(req.Context(), &auth.User{ID: "admin1", Role: "admin"})
	return req.WithContext(ctx)
}

func disabledStatusFn() (bool, string) { return false, "cloud sync disabled" }
func enabledStatusFn() (bool, string)  { return true, "" }
func enabledTrue() bool                { return true }
func enabledFalse() bool               { return false }

func TestHandler_Status_Disabled(t *testing.T) {
	h := NewHandler(nil, nil, nil, "", disabledStatusFn, enabledFalse, zap.NewNop())
	w := httptest.NewRecorder()
	h.status(w, adminRequest(http.MethodGet, "/status", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(body) != 2 {
		t.Fatalf("expected exactly 2 keys (enabled, reason) when disabled, got %v", body)
	}
	if enabled, _ := body["enabled"].(bool); enabled {
		t.Fatalf("expected enabled=false, got %v", body["enabled"])
	}
	if reason, _ := body["reason"].(string); reason != "cloud sync disabled" {
		t.Fatalf("expected reason to match, got %v", body["reason"])
	}
}

func TestHandler_Status_Enabled_Schema(t *testing.T) {
	store := newFakeStateStore()
	store.states["rec-1"] = &SyncState{RecordingID: "rec-1", Status: "synced"}
	store.states["rec-2"] = &SyncState{RecordingID: "rec-2", Status: "pending"}
	store.states["rec-3"] = &SyncState{RecordingID: "rec-3", Status: "syncing"}
	store.states["rec-4"] = &SyncState{RecordingID: "rec-4", Status: "error"}

	h := NewHandler(nil, nil, store, "myremote", enabledStatusFn, enabledTrue, zap.NewNop())
	w := httptest.NewRecorder()
	h.status(w, adminRequest(http.MethodGet, "/status", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if enabled, _ := body["enabled"].(bool); !enabled {
		t.Fatalf("expected enabled=true, got %v", body["enabled"])
	}
	if rc, _ := body["remote_configured"].(bool); !rc {
		t.Fatalf("expected remote_configured=true, got %v", body["remote_configured"])
	}
	if name, _ := body["remote_name"].(string); name != "myremote" {
		t.Fatalf("expected remote_name=myremote, got %v", body["remote_name"])
	}
	counts, ok := body["counts"].(map[string]any)
	if !ok {
		t.Fatalf("expected counts object, got %T: %v", body["counts"], body["counts"])
	}
	if counts["total"].(float64) != 4 {
		t.Fatalf("expected total=4, got %v", counts["total"])
	}
	if counts["synced"].(float64) != 1 || counts["pending"].(float64) != 1 ||
		counts["syncing"].(float64) != 1 || counts["error"].(float64) != 1 {
		t.Fatalf("unexpected counts breakdown: %v", counts)
	}
	if inProgress, _ := body["in_progress"].(bool); !inProgress {
		t.Fatalf("expected in_progress=true (syncing count > 0), got %v", body["in_progress"])
	}
	if _, ok := body["last_reconcile_at"]; !ok {
		t.Fatalf("expected last_reconcile_at key present (even if null), got %v", body)
	}

	// No secret-shaped values anywhere in the response body.
	lower := strings.ToLower(w.Body.String())
	for _, suspicious := range []string{"rclone.conf", "token", "secret", "password", "access_key", "bearer"} {
		if strings.Contains(lower, suspicious) {
			t.Fatalf("status response leaked suspicious substring %q: %s", suspicious, w.Body.String())
		}
	}
}

func TestHandler_FailedSyncs_Disabled(t *testing.T) {
	h := NewHandler(nil, nil, nil, "", disabledStatusFn, enabledFalse, zap.NewNop())
	w := httptest.NewRecorder()
	h.failedSyncs(w, adminRequest(http.MethodGet, "/failed", nil))

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}

func TestHandler_FailedSyncs_ResponseShapeAndRetryStatus(t *testing.T) {
	now := time.Now()
	nextAttempt := now.Add(time.Hour)
	errClassA := "copy_failed"
	errMsgA := "disk full"
	errClassB := "verify_failed"
	errMsgB := "size mismatch"

	store := newFakeStateStore()
	store.listFailedSyncsResult = []FailedSync{
		{
			RecordingID:   "rec-exhausted",
			Title:         "Show A",
			Status:        "error",
			ErrorClass:    &errClassA,
			Error:         &errMsgA,
			Attempts:      5,
			LastAttemptAt: &now,
			NextAttemptAt: &nextAttempt,
		},
		{
			RecordingID:   "rec-retrying",
			Title:         "Show B",
			Status:        "error",
			ErrorClass:    &errClassB,
			Error:         &errMsgB,
			Attempts:      1,
			LastAttemptAt: &now,
			NextAttemptAt: &nextAttempt,
		},
		{
			RecordingID:   "rec-midretry",
			Title:         "Show C",
			Status:        "syncing",
			ErrorClass:    &errClassA,
			Error:         &errMsgA,
			Attempts:      2,
			LastAttemptAt: &now,
			NextAttemptAt: nil,
		},
	}

	svc := &Service{cfg: Config{MaxAttempts: 5}}
	h := NewHandler(svc, nil, store, "remote", enabledStatusFn, enabledTrue, zap.NewNop())

	w := httptest.NewRecorder()
	h.failedSyncs(w, adminRequest(http.MethodGet, "/failed", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var body struct {
		FailedSyncs []map[string]any `json:"failed_syncs"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(body.FailedSyncs) != 3 {
		t.Fatalf("expected 3 rows, got %d: %v", len(body.FailedSyncs), body.FailedSyncs)
	}

	row0 := body.FailedSyncs[0]
	if row0["recording_id"] != "rec-exhausted" {
		t.Fatalf("expected rec-exhausted first, got %v", row0["recording_id"])
	}
	if row0["title"] != "Show A" {
		t.Fatalf("expected title Show A, got %v", row0["title"])
	}
	if row0["error_class"] != errClassA {
		t.Fatalf("expected error_class copy_failed, got %v", row0["error_class"])
	}
	if row0["error"] != errMsgA {
		t.Fatalf("expected error disk full, got %v", row0["error"])
	}
	if row0["retry_status"] != "exhausted" {
		t.Fatalf("expected retry_status exhausted for attempts=5/max=5, got %v", row0["retry_status"])
	}

	row1 := body.FailedSyncs[1]
	if row1["retry_status"] != "retrying" {
		t.Fatalf("expected retry_status retrying for attempts=1/max=5, got %v", row1["retry_status"])
	}

	row2 := body.FailedSyncs[2]
	if row2["retry_status"] != "retrying_now" {
		t.Fatalf("expected retry_status retrying_now for mid-retry (status=syncing) row, got %v", row2["retry_status"])
	}
	if row2["next_attempt_at"] != nil {
		t.Fatalf("expected next_attempt_at nil for mid-retry row, got %v", row2["next_attempt_at"])
	}
}

func TestHandler_FailedSyncs_StoreErrorReturns500(t *testing.T) {
	store := newFakeStateStore()
	store.listFailedSyncsErr = errors.New("boom")

	h := NewHandler(nil, nil, store, "remote", enabledStatusFn, enabledTrue, zap.NewNop())
	w := httptest.NewRecorder()
	h.failedSyncs(w, adminRequest(http.MethodGet, "/failed", nil))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHandler_Run_Disabled(t *testing.T) {
	h := NewHandler(nil, nil, nil, "", disabledStatusFn, enabledFalse, zap.NewNop())
	w := httptest.NewRecorder()
	h.run(w, adminRequest(http.MethodPost, "/run", nil))

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}

func TestHandler_Run_Enabled_AsyncSingleReconcile(t *testing.T) {
	store := newFakeStateStore()
	lister := &fakeLister{candidates: nil} // empty -> Reconcile's loop stops after 1 slow ListAllForSync call
	storage, _ := recordings.NewLocalStorage(t.TempDir())
	svc := NewService(store, &fakeRemoteClient{}, lister, storage, zap.NewNop(), Config{
		MaxWorkers: 1, MaxAttempts: 5, LeaseTTL: time.Minute, BackoffBase: time.Second,
	})

	h := NewHandler(svc, &fakeRemoteClient{}, store, "remote", enabledStatusFn, enabledTrue, zap.NewNop())

	start := time.Now()
	w := httptest.NewRecorder()
	h.run(w, adminRequest(http.MethodPost, "/run", nil))
	elapsed := time.Since(start)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", w.Code)
	}
	// fakeLister.ListAllForSync sleeps 100ms; the handler must return long before that.
	if elapsed >= 80*time.Millisecond {
		t.Fatalf("expected /run to return immediately (async), took %v", elapsed)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&lister.calls) >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if calls := atomic.LoadInt32(&lister.calls); calls != 1 {
		t.Fatalf("expected exactly 1 ListAllForSync call from the single Reconcile pass, got %d", calls)
	}

	// last_reconcile_at should get populated once the background goroutine finishes.
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if h.getLastReconcileAt() != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if h.getLastReconcileAt() == nil {
		t.Fatalf("expected last_reconcile_at to be set after async Reconcile completes")
	}
}

func TestHandler_Test_Disabled(t *testing.T) {
	h := NewHandler(nil, nil, nil, "", disabledStatusFn, enabledFalse, zap.NewNop())
	w := httptest.NewRecorder()
	h.test(w, adminRequest(http.MethodPost, "/test", nil))

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}

func TestHandler_Test_Success(t *testing.T) {
	client := &handlerFakeRemoteClient{remoteExistsFn: func(ctx context.Context) (bool, error) { return true, nil }}
	h := NewHandler(nil, client, nil, "remote", enabledStatusFn, enabledTrue, zap.NewNop())
	w := httptest.NewRecorder()
	h.test(w, adminRequest(http.MethodPost, "/test", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if ok, _ := body["ok"].(bool); !ok {
		t.Fatalf("expected ok=true, got %v", body)
	}
}

func TestHandler_Test_NegativeResultIsStill200(t *testing.T) {
	client := &handlerFakeRemoteClient{remoteExistsFn: func(ctx context.Context) (bool, error) { return false, nil }}
	h := NewHandler(nil, client, nil, "remote", enabledStatusFn, enabledTrue, zap.NewNop())
	w := httptest.NewRecorder()
	h.test(w, adminRequest(http.MethodPost, "/test", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 even for a negative test result, got %d", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if ok, _ := body["ok"].(bool); ok {
		t.Fatalf("expected ok=false, got %v", body)
	}
	if _, hasReason := body["reason"]; !hasReason {
		t.Fatalf("expected a reason field on negative result, got %v", body)
	}
}

func TestHandler_Remote_Disabled(t *testing.T) {
	h := NewHandler(nil, nil, nil, "", disabledStatusFn, enabledFalse, zap.NewNop())
	w := httptest.NewRecorder()
	body := strings.NewReader("[remote]\ntype = s3")
	h.remote(w, adminRequest(http.MethodPost, "/remote", body))

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}

func TestHandler_Remote_PlainTextSuccess(t *testing.T) {
	client := &handlerFakeRemoteClient{}
	h := NewHandler(nil, client, nil, "remote", enabledStatusFn, enabledTrue, zap.NewNop())
	w := httptest.NewRecorder()
	body := strings.NewReader("[remote]\ntype = s3")
	req := adminRequest(http.MethodPost, "/remote", body)
	h.remote(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
	if client.lastBlock != "[remote]\ntype = s3" {
		t.Fatalf("expected raw body passed through to WriteRemoteConfig, got %q", client.lastBlock)
	}
}

func TestHandler_Remote_JSONBodySuccess(t *testing.T) {
	client := &handlerFakeRemoteClient{}
	h := NewHandler(nil, client, nil, "remote", enabledStatusFn, enabledTrue, zap.NewNop())
	w := httptest.NewRecorder()
	body := strings.NewReader(`{"config":"[remote]\ntype = s3"}`)
	req := adminRequest(http.MethodPost, "/remote", body)
	req.Header.Set("Content-Type", "application/json")
	h.remote(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
	if client.lastBlock != "[remote]\ntype = s3" {
		t.Fatalf("expected extracted config field passed to WriteRemoteConfig, got %q", client.lastBlock)
	}
}

func TestHandler_Remote_Malformed(t *testing.T) {
	client := &handlerFakeRemoteClient{
		writeConfigFn: func(ctx context.Context, block string) error {
			return ErrRemotePathConflict // any error stands in for a validation failure here
		},
	}
	h := NewHandler(nil, client, nil, "remote", enabledStatusFn, enabledTrue, zap.NewNop())
	w := httptest.NewRecorder()
	body := strings.NewReader("[wrong-section]\ntype = s3")
	req := adminRequest(http.MethodPost, "/remote", body)
	h.remote(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	var errBody map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &errBody); err != nil {
		t.Fatalf("expected JSON error body: %v", err)
	}
	if errBody["error"] == "" {
		t.Fatalf("expected non-empty error field, got %v", errBody)
	}
}

func newRetryRouter(h *Handler) http.Handler {
	r := chi.NewRouter()
	r.Post("/failed/{recordingID}/retry", h.retry)
	return r
}

func TestHandler_Retry_Disabled(t *testing.T) {
	h := NewHandler(nil, nil, nil, "", disabledStatusFn, enabledFalse, zap.NewNop())
	r := newRetryRouter(h)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, adminRequest(http.MethodPost, "/failed/rec-1/retry", nil))

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}

func TestHandler_Retry_NotFoundRecording(t *testing.T) {
	store := newFakeStateStore()
	h := NewHandler(nil, nil, store, "remote", enabledStatusFn, enabledTrue, zap.NewNop())
	r := newRetryRouter(h)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, adminRequest(http.MethodPost, "/failed/rec-missing/retry", nil))

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("expected JSON error body: %v", err)
	}
	if body["error"] == "" {
		t.Fatalf("expected non-empty error field, got %v", body)
	}
}

func TestHandler_Retry_StoreGetErrorReturns500(t *testing.T) {
	store := &fakeGetErrStateStore{fakeStateStore: newFakeStateStore(), getErr: errors.New("boom")}
	h := NewHandler(nil, nil, store, "remote", enabledStatusFn, enabledTrue, zap.NewNop())
	r := newRetryRouter(h)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, adminRequest(http.MethodPost, "/failed/rec-1/retry", nil))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// TestHandler_Retry_Success_AcceptedAndBypassesMaxAttempts covers the core
// happy path: a Failed Sync whose attempts already meets/exceeds
// max_attempts (Exhausted) still gets retried -- the endpoint responds
// immediately, and the background dispatch (via Service.ForceRetry /
// StateStore.ClaimRetry) bypasses the attempts < max_attempts gate,
// increments attempts further, and completes the sync.
func TestHandler_Retry_Success_AcceptedAndBypassesMaxAttempts(t *testing.T) {
	store := newFakeStateStore()
	now := time.Now()
	errClass := "copy_failed"
	store.states["rec-1"] = &SyncState{
		RecordingID:   "rec-1",
		RemotePath:    "path.mp3",
		Status:        "error",
		ErrorClass:    &errClass,
		Attempts:      6, // already >= MaxAttempts below (Exhausted)
		LastAttemptAt: &now,
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "rec-1.mp3"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	lister := &fakeLister{
		getByIDFn: func(ctx context.Context, id string) (*recordings.Recording, error) {
			return &recordings.Recording{ID: id, StoragePath: "rec-1.mp3"}, nil
		},
	}
	client := &fakeRemoteClient{
		statFunc: func(ctx context.Context, remotePath string) (int64, bool, error) {
			return int64(len("hello")), true, nil
		},
	}
	storage := &fakeStorage{dir: dir}
	svc := NewService(store, client, lister, storage, zap.NewNop(), Config{
		MaxWorkers: 1, MaxAttempts: 5, LeaseTTL: time.Minute, BackoffBase: time.Millisecond,
	})

	h := NewHandler(svc, client, store, "remote", enabledStatusFn, enabledTrue, zap.NewNop())
	r := newRetryRouter(h)

	start := time.Now()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, adminRequest(http.MethodPost, "/failed/rec-1/retry", nil))
	elapsed := time.Since(start)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", w.Code, w.Body.String())
	}
	if elapsed >= 80*time.Millisecond {
		t.Fatalf("expected retry to return immediately (async), took %v", elapsed)
	}

	deadline := time.Now().Add(2 * time.Second)
	var state *SyncState
	for time.Now().Before(deadline) {
		state, _ = store.Get(context.Background(), "rec-1")
		if state != nil && state.Status == "synced" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if state == nil || state.Status != "synced" {
		t.Fatalf("expected retry to bypass max_attempts and eventually succeed, got %+v", state)
	}
	if state.Attempts != 7 {
		t.Fatalf("expected attempts to keep incrementing honestly (6 -> 7) despite exceeding max_attempts=5, got %d", state.Attempts)
	}
}

// TestHandler_Retry_AlreadyInProgress_RaceIsSafeNoOp covers the
// already-in-progress race: a Retry click against a row that is currently
// mid-retry under a live lease (retry_status="retrying_now") must still
// respond 202 immediately, and the background ClaimRetry call must no-op
// rather than stomping on the in-flight attempt (owner/attempts unchanged).
func TestHandler_Retry_AlreadyInProgress_RaceIsSafeNoOp(t *testing.T) {
	store := newFakeStateStore()
	futureLease := time.Now().Add(time.Hour)
	errClass := "copy_failed"
	owner := "other-in-flight-owner"
	store.states["rec-1"] = &SyncState{
		RecordingID:    "rec-1",
		RemotePath:     "path.mp3",
		Status:         "syncing",
		ErrorClass:     &errClass,
		Attempts:       2,
		LeaseOwner:     &owner,
		LeaseExpiresAt: &futureLease,
	}

	lister := &fakeLister{
		getByIDFn: func(ctx context.Context, id string) (*recordings.Recording, error) {
			return &recordings.Recording{ID: id, StoragePath: "rec-1.mp3"}, nil
		},
	}
	svc := NewService(store, &fakeRemoteClient{}, lister, &fakeStorage{dir: t.TempDir()}, zap.NewNop(), Config{
		MaxWorkers: 1, MaxAttempts: 5, LeaseTTL: time.Minute, BackoffBase: time.Millisecond,
	})
	h := NewHandler(svc, &fakeRemoteClient{}, store, "remote", enabledStatusFn, enabledTrue, zap.NewNop())
	r := newRetryRouter(h)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, adminRequest(http.MethodPost, "/failed/rec-1/retry", nil))

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202 even when a retry is already in flight, got %d: %s", w.Code, w.Body.String())
	}

	// Wait for the background ForceRetry call to actually run (and no-op).
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		store.mu.Lock()
		claims := store.claims
		store.mu.Unlock()
		if claims >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	// Give the (already-observed) claim attempt a moment to fully settle.
	time.Sleep(20 * time.Millisecond)

	state, _ := store.Get(context.Background(), "rec-1")
	if state == nil {
		t.Fatalf("expected state to still exist")
	}
	if state.Attempts != 2 {
		t.Fatalf("expected attempts to remain unchanged at 2 (in-flight retry's live lease must not be stolen), got %d", state.Attempts)
	}
	if state.LeaseOwner == nil || *state.LeaseOwner != owner {
		t.Fatalf("expected original in-flight owner to remain, got %v", state.LeaseOwner)
	}
	if state.Status != "syncing" {
		t.Fatalf("expected status to remain 'syncing', got %q", state.Status)
	}
}

// fakeGetErrStateStore wraps fakeStateStore to force Get to return an error,
// for exercising the retry handler's 500 path.
type fakeGetErrStateStore struct {
	*fakeStateStore
	getErr error
}

func (f *fakeGetErrStateStore) Get(ctx context.Context, recordingID string) (*SyncState, error) {
	return nil, f.getErr
}

func TestHandler_Routes_SelfAppliesRequireAdmin(t *testing.T) {
	h := NewHandler(nil, nil, nil, "", disabledStatusFn, enabledFalse, zap.NewNop())
	r := chi.NewRouter()
	h.Routes(r)

	// No admin user in context -> RequireAdmin must reject with 403, proving
	// each route self-applies auth.RequireAdmin per Must-Do.
	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without admin user in context, got %d", w.Code)
	}

	// With an admin user in context, the route runs normally.
	req = adminRequest(http.MethodGet, "/status", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with admin user in context, got %d", w.Code)
	}
}

// TestHandler_RemoteAndTest_UsableBeforeFullReadiness locks in the fix for
// the critical connect-flow bug: on first-time setup CLOUD_SYNC_ENABLED=true
// but the remote doesn't exist in rclone.conf yet, so the full statusFn probe
// reports disabled ("remote not configured"). /remote and /test must stay
// reachable in that state (gated on enabledFn only) since they're literally
// how an admin fixes "remote not configured" — only /status and /run should
// honor the full probe's disabled verdict.
func TestHandler_RemoteAndTest_UsableBeforeFullReadiness(t *testing.T) {
	remoteNotConfiguredFn := func() (bool, string) { return false, "remote 'x' not configured" }
	client := &handlerFakeRemoteClient{remoteExistsFn: func(ctx context.Context) (bool, error) { return false, nil }}
	h := NewHandler(nil, client, nil, "x", remoteNotConfiguredFn, enabledTrue, zap.NewNop())

	// /status and /run still correctly report disabled via the full probe.
	w := httptest.NewRecorder()
	h.status(w, adminRequest(http.MethodGet, "/status", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status: expected 200, got %d", w.Code)
	}
	var statusBody map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &statusBody); err != nil {
		t.Fatalf("status: invalid JSON: %v", err)
	}
	if enabled, _ := statusBody["enabled"].(bool); enabled {
		t.Fatalf("status: expected enabled=false (remote not configured), got %v", statusBody)
	}

	w = httptest.NewRecorder()
	h.run(w, adminRequest(http.MethodPost, "/run", nil))
	if w.Code != http.StatusConflict {
		t.Fatalf("run: expected 409 (nothing to sync to yet), got %d", w.Code)
	}

	// /test must NOT 409 here -- it should actually run RemoteExists and
	// report a legitimate negative result.
	w = httptest.NewRecorder()
	h.test(w, adminRequest(http.MethodPost, "/test", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("test: expected 200 (not blocked by full-readiness probe), got %d", w.Code)
	}
	var testBody map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &testBody); err != nil {
		t.Fatalf("test: invalid JSON: %v", err)
	}
	if ok, _ := testBody["ok"].(bool); ok {
		t.Fatalf("test: expected ok=false (remote genuinely not reachable yet), got %v", testBody)
	}

	// /remote must NOT 409 here -- it's how the admin fixes the state.
	w = httptest.NewRecorder()
	body := strings.NewReader("[x]\ntype = s3")
	h.remote(w, adminRequest(http.MethodPost, "/remote", body))
	if w.Code != http.StatusNoContent {
		t.Fatalf("remote: expected 204 (not blocked by full-readiness probe), got %d: %s", w.Code, w.Body.String())
	}
	if client.lastBlock != "[x]\ntype = s3" {
		t.Fatalf("remote: expected config block to reach WriteRemoteConfig, got %q", client.lastBlock)
	}
}

func TestHandler_Remote_OversizedBodyRejected(t *testing.T) {
	client := &handlerFakeRemoteClient{}
	h := NewHandler(nil, client, nil, "remote", enabledStatusFn, enabledTrue, zap.NewNop())

	oversized := strings.NewReader("[remote]\n# " + strings.Repeat("a", maxRemoteConfigBytes+1))
	w := httptest.NewRecorder()
	h.remote(w, adminRequest(http.MethodPost, "/remote", oversized))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for oversized body, got %d", w.Code)
	}
	if client.lastBlock != "" {
		t.Fatalf("expected WriteRemoteConfig to never be called with a truncated body, got %q (len %d)", client.lastBlock, len(client.lastBlock))
	}
	var errBody map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &errBody); err != nil {
		t.Fatalf("expected JSON error body: %v", err)
	}
	if errBody["error"] == "" {
		t.Fatalf("expected non-empty error field, got %v", errBody)
	}
}
