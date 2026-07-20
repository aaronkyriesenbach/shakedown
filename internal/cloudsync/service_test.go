package cloudsync

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"

	"shakedown/internal/recordings"
)

type fakeStateStore struct {
	mu           sync.Mutex
	states       map[string]*SyncState
	claims       int
	markErrorCnt int
	markSyncCnt  int
	heartbeat    func(ctx context.Context, recordingID, owner string, ttl time.Duration) (int64, error)

	listFailedSyncsResult []FailedSync
	listFailedSyncsErr    error
}

func newFakeStateStore() *fakeStateStore {
	return &fakeStateStore{
		states: make(map[string]*SyncState),
	}
}

func (f *fakeStateStore) ClaimNew(ctx context.Context, recordingID, remotePath, owner string, leaseTTL time.Duration, maxAttempts int) (ClaimResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.claims++

	if s, ok := f.states[recordingID]; ok {
		if s.Attempts >= maxAttempts {
			return ClaimResult{Claimed: false}, nil
		}
		if s.Status == "syncing" && s.LeaseExpiresAt != nil && time.Now().Before(*s.LeaseExpiresAt) {
			return ClaimResult{Claimed: false}, nil
		}
		if s.NextAttemptAt != nil && time.Now().Before(*s.NextAttemptAt) {
			return ClaimResult{Claimed: false}, nil
		}

		s.Status = "syncing"
		s.Attempts++
		ownerStr := owner
		s.LeaseOwner = &ownerStr
		expires := time.Now().Add(leaseTTL)
		s.LeaseExpiresAt = &expires
		return ClaimResult{Claimed: true, RemotePath: s.RemotePath, Attempts: s.Attempts}, nil
	}

	expires := time.Now().Add(leaseTTL)
	ownerStr := owner
	f.states[recordingID] = &SyncState{
		Status:         "syncing",
		Attempts:       1,
		LeaseOwner:     &ownerStr,
		RemotePath:     remotePath,
		LeaseExpiresAt: &expires,
	}
	return ClaimResult{Claimed: true, RemotePath: remotePath, Attempts: 1}, nil
}

func (f *fakeStateStore) ClaimRetry(ctx context.Context, recordingID, owner string, leaseTTL time.Duration) (ClaimResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.claims++

	s, ok := f.states[recordingID]
	if !ok {
		return ClaimResult{Claimed: false}, nil
	}
	if s.Status == "syncing" && s.LeaseExpiresAt != nil && time.Now().Before(*s.LeaseExpiresAt) {
		return ClaimResult{Claimed: false}, nil
	}
	if s.Status != "error" && s.Status != "syncing" {
		return ClaimResult{Claimed: false}, nil
	}

	s.Status = "syncing"
	s.Attempts++
	ownerStr := owner
	s.LeaseOwner = &ownerStr
	expires := time.Now().Add(leaseTTL)
	s.LeaseExpiresAt = &expires
	return ClaimResult{Claimed: true, RemotePath: s.RemotePath, Attempts: s.Attempts}, nil
}

func (f *fakeStateStore) Get(ctx context.Context, recordingID string) (*SyncState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.states[recordingID], nil
}

func (f *fakeStateStore) MarkSynced(ctx context.Context, recordingID, owner, remoteFileID string, remoteSize int64) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.states[recordingID]
	if !ok || s.LeaseOwner == nil || *s.LeaseOwner != owner {
		return 0, nil
	}
	f.markSyncCnt++
	s.Status = "synced"
	s.LeaseOwner = nil
	s.LeaseExpiresAt = nil
	return 1, nil
}

func (f *fakeStateStore) MarkError(ctx context.Context, recordingID, owner, errClass, errMsg string, nextAttemptAt time.Time) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.states[recordingID]
	if !ok || s.LeaseOwner == nil || *s.LeaseOwner != owner {
		return 0, nil
	}
	f.markErrorCnt++
	s.Status = "error"
	s.ErrorClass = &errClass
	s.LeaseOwner = nil
	s.LeaseExpiresAt = nil
	s.NextAttemptAt = &nextAttemptAt
	return 1, nil
}

func (f *fakeStateStore) Heartbeat(ctx context.Context, recordingID, owner string, ttl time.Duration) (int64, error) {
	if f.heartbeat != nil {
		return f.heartbeat(ctx, recordingID, owner, ttl)
	}
	return 1, nil
}

func (f *fakeStateStore) RecoverExpiredLeases(ctx context.Context) (int64, error) {
	return 0, nil
}

func (f *fakeStateStore) CountByStatus(ctx context.Context) (map[string]int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	counts := make(map[string]int)
	for _, s := range f.states {
		counts[s.Status]++
	}
	return counts, nil
}

func (f *fakeStateStore) ListFailedSyncs(ctx context.Context) ([]FailedSync, error) {
	if f.listFailedSyncsErr != nil {
		return nil, f.listFailedSyncsErr
	}
	return f.listFailedSyncsResult, nil
}

type fakeRemoteClient struct {
	copyFunc func(ctx context.Context, localAbsPath, remotePath string) error
	statFunc func(ctx context.Context, remotePath string) (int64, bool, error)
}

func (f *fakeRemoteClient) Version(ctx context.Context) (string, error)               { return "", nil }
func (f *fakeRemoteClient) RemoteExists(ctx context.Context) (bool, error)            { return true, nil }
func (f *fakeRemoteClient) WriteRemoteConfig(ctx context.Context, block string) error { return nil }
func (f *fakeRemoteClient) Copy(ctx context.Context, localAbsPath, remotePath string) error {
	if f.copyFunc != nil {
		return f.copyFunc(ctx, localAbsPath, remotePath)
	}
	return nil
}
func (f *fakeRemoteClient) StatSize(ctx context.Context, remotePath string) (int64, bool, error) {
	if f.statFunc != nil {
		return f.statFunc(ctx, remotePath)
	}
	return 0, false, nil
}

type fakeLister struct {
	candidates []recordings.SyncCandidate
	calls      int32
	getByIDFn  func(ctx context.Context, id string) (*recordings.Recording, error)
}

func (f *fakeLister) ListAllForSync(ctx context.Context, afterID string, limit int) ([]recordings.SyncCandidate, error) {
	atomic.AddInt32(&f.calls, 1)
	time.Sleep(100 * time.Millisecond) // artificially block to ensure concurrency test overlap
	if afterID == "" {
		return f.candidates, nil
	}
	return nil, nil
}

func (f *fakeLister) GetByID(ctx context.Context, id string) (*recordings.Recording, error) {
	if f.getByIDFn != nil {
		return f.getByIDFn(ctx, id)
	}
	return nil, nil
}

type fakeStorage struct {
	dir string
}

func (f *fakeStorage) FullPath(path string) (string, error) {
	return filepath.Join(f.dir, path), nil
}

func TestService_ConcurrentReconcile(t *testing.T) {
	lister := &fakeLister{}
	cfg := Config{MaxWorkers: 1}
	svc := NewService(newFakeStateStore(), &fakeRemoteClient{}, lister, &fakeStorage{}, zap.NewNop(), cfg)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _ = svc.Reconcile(context.Background()) }()
	go func() { defer wg.Done(); _ = svc.Reconcile(context.Background()) }()
	wg.Wait()

	if calls := atomic.LoadInt32(&lister.calls); calls != 1 {
		t.Errorf("expected exactly 1 call to lister, got %d", calls)
	}
}

func TestService_Shutdown(t *testing.T) {
	store := newFakeStateStore()
	lister := &fakeLister{
		candidates: []recordings.SyncCandidate{{ID: "1", StoragePath: "test.mp3", FileSizeBytes: 100}},
	}

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.mp3"), make([]byte, 100), 0644)

	copyStarted := make(chan struct{})
	client := &fakeRemoteClient{
		copyFunc: func(ctx context.Context, local, remote string) error {
			close(copyStarted)
			<-ctx.Done()
			return ctx.Err()
		},
	}

	cfg := Config{MaxWorkers: 1, LeaseTTL: time.Hour, Root: "root", PathTemplate: "{id}.mp3"}
	svc := NewService(store, client, lister, &fakeStorage{dir}, zap.NewNop(), cfg)

	_ = svc.Reconcile(context.Background())
	<-copyStarted

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	start := time.Now()
	svc.Shutdown(shutdownCtx)
	if time.Since(start) > 2*time.Second {
		t.Errorf("shutdown took too long")
	}
}

func TestService_HeartbeatLoss(t *testing.T) {
	store := newFakeStateStore()

	// Fail on second heartbeat
	var hbCount int32
	store.heartbeat = func(ctx context.Context, id, owner string, ttl time.Duration) (int64, error) {
		if atomic.AddInt32(&hbCount, 1) > 1 {
			return 0, errors.New("lease lost")
		}
		return 1, nil
	}

	lister := &fakeLister{
		candidates: []recordings.SyncCandidate{{ID: "1", StoragePath: "test.mp3", FileSizeBytes: 100}},
	}

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.mp3"), make([]byte, 100), 0644)

	client := &fakeRemoteClient{
		copyFunc: func(ctx context.Context, local, remote string) error {
			time.Sleep(100 * time.Millisecond) // wait for heartbeat to fail
			return nil
		},
	}

	cfg := Config{MaxWorkers: 1, LeaseTTL: 10 * time.Millisecond, Root: "root", PathTemplate: "{id}.mp3"}
	svc := NewService(store, client, lister, &fakeStorage{dir}, zap.NewNop(), cfg)

	_ = svc.Reconcile(context.Background())

	// Wait for processing to finish
	svc.wg.Wait()

	store.mu.Lock()
	defer store.mu.Unlock()
	if store.markErrorCnt > 0 || store.markSyncCnt > 0 {
		t.Errorf("expected no terminal state writes, got error:%d sync:%d", store.markErrorCnt, store.markSyncCnt)
	}
}

func TestService_IdempotentReconcile(t *testing.T) {
	store := newFakeStateStore()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "eligible.mp3"), make([]byte, 100), 0644)

	lister := &fakeLister{
		candidates: []recordings.SyncCandidate{
			{ID: "eligible", StoragePath: "eligible.mp3", FileSizeBytes: 100},
		},
	}

	var copyCalls int32
	client := &fakeRemoteClient{
		copyFunc: func(ctx context.Context, local, remote string) error {
			atomic.AddInt32(&copyCalls, 1)
			return nil
		},
		statFunc: func(ctx context.Context, remotePath string) (int64, bool, error) {
			return 100, true, nil
		},
	}

	cfg := Config{MaxWorkers: 1, LeaseTTL: time.Hour, Root: "root", PathTemplate: "{id}.mp3"}
	svc := NewService(store, client, lister, &fakeStorage{dir}, zap.NewNop(), cfg)

	_ = svc.Reconcile(context.Background())
	svc.wg.Wait()

	if atomic.LoadInt32(&copyCalls) != 1 {
		t.Fatalf("expected 1 copy call, got %d", copyCalls)
	}

	// Second reconcile
	_ = svc.Reconcile(context.Background())
	svc.wg.Wait()

	if atomic.LoadInt32(&copyCalls) != 1 {
		t.Fatalf("expected 1 copy call after second reconcile, got %d", copyCalls)
	}
}

func TestService_StoredPathUsed(t *testing.T) {
	store := newFakeStateStore()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "file.mp3"), make([]byte, 100), 0644)

	// Start with Title A
	lister := &fakeLister{
		candidates: []recordings.SyncCandidate{
			{ID: "123", Title: "Title A", StoragePath: "file.mp3", FileSizeBytes: 100},
		},
	}

	var copyCalls int32
	var lastRemote string
	var clientMu sync.Mutex

	client := &fakeRemoteClient{
		copyFunc: func(ctx context.Context, local, remote string) error {
			atomic.AddInt32(&copyCalls, 1)
			clientMu.Lock()
			lastRemote = remote
			clientMu.Unlock()
			return errors.New("fail first time to trigger retry")
		},
		statFunc: func(ctx context.Context, remotePath string) (int64, bool, error) {
			return 100, true, nil
		},
	}

	cfg := Config{MaxWorkers: 1, LeaseTTL: time.Hour, Root: "root", PathTemplate: "{title}.mp3"}
	svc := NewService(store, client, lister, &fakeStorage{dir}, zap.NewNop(), cfg)

	// First reconcile
	_ = svc.Reconcile(context.Background())
	svc.wg.Wait()

	clientMu.Lock()
	if lastRemote != "root/Title A.mp3" {
		t.Fatalf("expected first remote path 'root/Title A.mp3', got %q", lastRemote)
	}
	clientMu.Unlock()

	// Change Title to B in lister
	lister.candidates[0].Title = "Title B"

	// Clear NextAttemptAt so we can claim again immediately
	store.mu.Lock()
	store.states["123"].NextAttemptAt = nil
	store.mu.Unlock()

	// Let second copy succeed
	client.copyFunc = func(ctx context.Context, local, remote string) error {
		atomic.AddInt32(&copyCalls, 1)
		clientMu.Lock()
		lastRemote = remote
		clientMu.Unlock()
		return nil
	}

	// Second reconcile
	_ = svc.Reconcile(context.Background())
	svc.wg.Wait()

	clientMu.Lock()
	if lastRemote != "root/Title A.mp3" {
		t.Fatalf("expected second remote path to STILL be 'root/Title A.mp3', got %q", lastRemote)
	}
	clientMu.Unlock()
}

func TestService_AcceptanceMatrix(t *testing.T) {
	store := newFakeStateStore()
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "eligible.mp3"), make([]byte, 100), 0644)
	os.WriteFile(filepath.Join(dir, "mismatch.mp3"), make([]byte, 100), 0644)

	store.states["synced"] = &SyncState{Status: "synced"}
	owner := "previous-owner"
	expires := time.Now().Add(-1 * time.Hour)
	store.states["missing"] = &SyncState{
		Status:         "syncing",
		Attempts:       1,
		LeaseOwner:     &owner,
		LeaseExpiresAt: &expires,
	}

	lister := &fakeLister{
		candidates: []recordings.SyncCandidate{
			{ID: "eligible", StoragePath: "eligible.mp3", FileSizeBytes: 100},
			{ID: "synced", StoragePath: "synced.mp3", FileSizeBytes: 100},
			{ID: "missing", StoragePath: "missing.mp3", FileSizeBytes: 100},
			{ID: "never_claimed_missing", StoragePath: "never_claimed.mp3", FileSizeBytes: 100},
			{ID: "mismatch", StoragePath: "mismatch.mp3", FileSizeBytes: 100},
		},
	}

	copies := make(chan string, 10)
	client := &fakeRemoteClient{
		copyFunc: func(ctx context.Context, local, remote string) error {
			copies <- remote
			return nil
		},
		statFunc: func(ctx context.Context, remotePath string) (int64, bool, error) {
			if remotePath == "root/mismatch.mp3" {
				return 99, true, nil // mismatch
			}
			return 100, true, nil
		},
	}

	cfg := Config{MaxWorkers: 4, LeaseTTL: time.Hour, Root: "root", PathTemplate: "{id}.mp3", MaxAttempts: 5}
	svc := NewService(store, client, lister, &fakeStorage{dir}, zap.NewNop(), cfg)

	_ = svc.Reconcile(context.Background())
	svc.wg.Wait()

	close(copies)

	store.mu.Lock()
	defer store.mu.Unlock()

	if store.states["missing"] == nil || store.states["missing"].Status != "error" || store.states["missing"].ErrorClass == nil || *store.states["missing"].ErrorClass != "local_missing" {
		t.Errorf("expected missing to have local_missing error")
	}

	if _, exists := store.states["never_claimed_missing"]; exists {
		t.Errorf("expected never_claimed_missing to NOT have a state row")
	}

	if store.states["mismatch"] == nil || store.states["mismatch"].Status != "error" || store.states["mismatch"].ErrorClass == nil || *store.states["mismatch"].ErrorClass != "verify_failed" {
		t.Errorf("expected mismatch to have verify_failed error")
	}

	if store.states["eligible"] == nil || store.states["eligible"].Status != "synced" {
		t.Errorf("expected eligible to be synced")
	}

	// Check original remote path is used
	foundEligiblePath := false
	for c := range copies {
		if c == "root/eligible.mp3" {
			foundEligiblePath = true
		}
	}
	if !foundEligiblePath {
		t.Errorf("eligible copy not performed to expected path")
	}
}
