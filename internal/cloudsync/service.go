package cloudsync

import (
	"context"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"shakedown/internal/recordings"
)

// Config represents the cloud sync config values needed by the Service
type Config struct {
	Root         string
	PathTemplate string
	Interval     time.Duration
	MaxWorkers   int
	MaxAttempts  int
	LeaseTTL     time.Duration
	BackoffBase  time.Duration
}

// RecordingsLister is a narrow interface for fetching recordings
type RecordingsLister interface {
	ListAllForSync(ctx context.Context, afterID string, limit int) ([]recordings.SyncCandidate, error)
	GetByID(ctx context.Context, id string) (*recordings.Recording, error)
}

// LocalStorageChecker is a narrow interface for resolving local file paths
type LocalStorageChecker interface {
	FullPath(path string) (string, error)
}

// Service is the orchestration layer for cloud sync.
type Service struct {
	store      StateStore
	client     RemoteClient
	lister     RecordingsLister
	storage    LocalStorageChecker
	logger     *zap.Logger
	cfg        Config
	leaseOwner string

	sem chan struct{}
	wg  sync.WaitGroup

	reconcileMutex sync.Mutex
	reconciling    bool

	workerCtx    context.Context
	workerCancel context.CancelFunc

	stopRecovery  chan struct{}
	stopScheduler chan struct{}
}

// NewService creates a new Service instance.
func NewService(
	store StateStore,
	client RemoteClient,
	lister RecordingsLister,
	storage LocalStorageChecker,
	logger *zap.Logger,
	cfg Config,
) *Service {
	if cfg.MaxWorkers <= 0 {
		cfg.MaxWorkers = 1
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Service{
		store:         store,
		client:        client,
		lister:        lister,
		storage:       storage,
		logger:        logger,
		cfg:           cfg,
		leaseOwner:    uuid.New().String(),
		sem:           make(chan struct{}, cfg.MaxWorkers),
		workerCtx:     ctx,
		workerCancel:  cancel,
		stopRecovery:  make(chan struct{}),
		stopScheduler: make(chan struct{}),
	}
}

// Reconcile processes all sync candidates.
func (s *Service) Reconcile(ctx context.Context) error {
	s.reconcileMutex.Lock()
	if s.reconciling {
		s.reconcileMutex.Unlock()
		return nil
	}
	s.reconciling = true
	s.reconcileMutex.Unlock()

	defer func() {
		s.reconcileMutex.Lock()
		s.reconciling = false
		s.reconcileMutex.Unlock()
	}()

	afterID := ""
	limit := 100

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if s.workerCtx.Err() != nil {
			return s.workerCtx.Err()
		}

		candidates, err := s.lister.ListAllForSync(ctx, afterID, limit)
		if err != nil {
			return err
		}

		if len(candidates) == 0 {
			break
		}

		for _, candidate := range candidates {
			afterID = candidate.ID
			s.processCandidate(ctx, candidate)
		}
	}
	return nil
}

// EnqueueRecording processes a single recording for sync.
func (s *Service) EnqueueRecording(ctx context.Context, recordingID string) error {
	rec, err := s.lister.GetByID(ctx, recordingID)
	if err != nil {
		return err
	}
	if rec == nil {
		return errors.New("recording not found")
	}

	cand := recordings.SyncCandidate{
		ID:            rec.ID,
		Title:         rec.Title,
		FileExt:       rec.FileExt,
		RecordedAt:    rec.RecordedAt,
		CreatedAt:     rec.CreatedAt,
		StoragePath:   rec.StoragePath,
		FileSizeBytes: rec.FileSizeBytes,
	}

	s.processCandidate(ctx, cand)
	return nil
}

func (s *Service) getBackoff(attempts int) time.Duration {
	if attempts < 0 {
		attempts = 0
	}
	// Limit multiplier so we don't overflow or wait forever
	if attempts > 10 {
		attempts = 10
	}
	multiplier := 1 << attempts
	return s.cfg.BackoffBase * time.Duration(multiplier)
}

func (s *Service) checkEligibility(candidate recordings.SyncCandidate) (string, bool, string) {
	if candidate.StoragePath == "" {
		return "", false, "empty storage path"
	}

	localAbsPath, err := s.storage.FullPath(candidate.StoragePath)
	if err != nil {
		return "", false, err.Error()
	}

	if _, err := os.Stat(localAbsPath); err != nil {
		return "", false, err.Error()
	}

	return localAbsPath, true, ""
}

func (s *Service) processCandidate(ctx context.Context, candidate recordings.SyncCandidate) {
	state, err := s.store.Get(ctx, candidate.ID)
	if err != nil {
		s.logger.Error("failed to get state", zap.Error(err), zap.String("recording_id", candidate.ID))
		return
	}
	if state != nil && state.Status == "synced" {
		return
	}

	hasExistingRow := state != nil

	localAbsPath, eligible, ineligibleReason := s.checkEligibility(candidate)

	if !eligible && !hasExistingRow {
		s.logger.Warn("skipping ineligible recording with no tracked state", zap.String("recording_id", candidate.ID))
		return
	}

	meta := RecordingMeta{
		ID:         candidate.ID,
		Title:      candidate.Title,
		FileExt:    candidate.FileExt,
		RecordedAt: candidate.RecordedAt,
		CreatedAt:  candidate.CreatedAt,
	}

	computedPath := RenderPath(s.cfg.PathTemplate, s.cfg.Root, meta)

	claim, err := s.store.ClaimNew(ctx, candidate.ID, computedPath, s.leaseOwner, s.cfg.LeaseTTL, s.cfg.MaxAttempts)
	if errors.Is(err, ErrRemotePathConflict) {
		shortID := candidate.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		computedPath = SuffixPath(computedPath, shortID)
		claim, err = s.store.ClaimNew(ctx, candidate.ID, computedPath, s.leaseOwner, s.cfg.LeaseTTL, s.cfg.MaxAttempts)
	}
	if err != nil {
		s.logger.Error("failed to claim", zap.Error(err), zap.String("recording_id", candidate.ID))
		return
	}

	if !claim.Claimed {
		return
	}

	if !eligible {
		nextAttempt := time.Now().Add(s.getBackoff(claim.Attempts))
		s.markErrorSafely(candidate.ID, "local_missing", ineligibleReason, nextAttempt)
		return
	}

	s.dispatchWorker(candidate.ID, localAbsPath, claim)
}

func (s *Service) dispatchWorker(recordingID string, localAbsPath string, claim ClaimResult) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		select {
		case s.sem <- struct{}{}:
		case <-s.workerCtx.Done():
			return
		}
		defer func() { <-s.sem }()

		// Re-stat immediately before copying
		fi, err := os.Stat(localAbsPath)
		if err != nil {
			nextAttempt := time.Now().Add(s.getBackoff(claim.Attempts))
			s.markErrorSafely(recordingID, "local_missing", err.Error(), nextAttempt)
			return
		}

		ctx, cancel := context.WithCancel(s.workerCtx)
		defer cancel()

		tickerTTL := s.cfg.LeaseTTL / 3
		if tickerTTL <= 0 {
			tickerTTL = time.Second // Fallback
		}
		ticker := time.NewTicker(tickerTTL)
		defer ticker.Stop()

		leaseLost := int32(0)

		// Heartbeat loop
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					hbCtx, hbCancel := context.WithTimeout(s.workerCtx, 5*time.Second)
					rows, err := s.store.Heartbeat(hbCtx, recordingID, s.leaseOwner, s.cfg.LeaseTTL)
					hbCancel()
					if err != nil || rows == 0 {
						atomic.StoreInt32(&leaseLost, 1)
						cancel()
						return
					}
				}
			}
		}()

		err = s.client.Copy(ctx, localAbsPath, claim.RemotePath)

		// If lease was lost or service shutting down, abort without terminal state
		if atomic.LoadInt32(&leaseLost) == 1 || s.workerCtx.Err() != nil {
			return
		}

		if err != nil {
			nextAttempt := time.Now().Add(s.getBackoff(claim.Attempts))
			s.markErrorSafely(recordingID, "copy_failed", err.Error(), nextAttempt)
			return
		}

		// Verify file size
		var found bool
		var size int64
		var statErr error
		for i := 0; i < 3; i++ {
			size, found, statErr = s.client.StatSize(ctx, claim.RemotePath)
			if statErr == nil && found && size == fi.Size() {
				break
			}
			time.Sleep(1 * time.Second)
		}

		if atomic.LoadInt32(&leaseLost) == 1 || s.workerCtx.Err() != nil {
			return
		}

		if statErr != nil || !found || size != fi.Size() {
			errMsg := "file size mismatch or not found"
			if statErr != nil {
				errMsg = statErr.Error()
			}
			nextAttempt := time.Now().Add(s.getBackoff(claim.Attempts))
			s.markErrorSafely(recordingID, "verify_failed", errMsg, nextAttempt)
			return
		}

		s.markSyncedSafely(recordingID, "", size)
	}()
}

func (s *Service) markErrorSafely(recordingID string, errClass, errMsg string, nextAttempt time.Time) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rows, err := s.store.MarkError(ctx, recordingID, s.leaseOwner, errClass, errMsg, nextAttempt)
	if err != nil {
		s.logger.Error("failed to mark error", zap.Error(err), zap.String("recording_id", recordingID))
	} else if rows == 0 {
		s.logger.Warn("mark error ignored: lease lost or stolen", zap.String("recording_id", recordingID))
	}
}

func (s *Service) markSyncedSafely(recordingID string, remoteFileID string, size int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rows, err := s.store.MarkSynced(ctx, recordingID, s.leaseOwner, remoteFileID, size)
	if err != nil {
		s.logger.Error("failed to mark synced", zap.Error(err), zap.String("recording_id", recordingID))
	} else if rows == 0 {
		s.logger.Warn("mark synced ignored: lease lost or stolen", zap.String("recording_id", recordingID))
	}
}

// StartScheduler starts the reconciliation loop.
func (s *Service) StartScheduler(interval time.Duration) {
	if interval <= 0 {
		interval = 1 * time.Hour
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		if err := s.Reconcile(s.workerCtx); err != nil {
			s.logger.Error("scheduler reconcile failed", zap.Error(err))
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-s.stopScheduler:
				return
			case <-ticker.C:
				if err := s.Reconcile(s.workerCtx); err != nil {
					s.logger.Error("scheduler reconcile failed", zap.Error(err))
				}
			}
		}
	}()
}

// StartRecoveryLoop starts the loop that clears expired leases.
func (s *Service) StartRecoveryLoop(interval time.Duration) {
	if interval <= 0 {
		interval = 1 * time.Hour
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		if _, err := s.store.RecoverExpiredLeases(s.workerCtx); err != nil {
			s.logger.Error("recovery loop failed", zap.Error(err))
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-s.stopRecovery:
				return
			case <-ticker.C:
				if _, err := s.store.RecoverExpiredLeases(s.workerCtx); err != nil {
					s.logger.Error("recovery loop failed", zap.Error(err))
				}
			}
		}
	}()
}

// Shutdown gracefully stops all processing.
func (s *Service) Shutdown(ctx context.Context) {
	close(s.stopScheduler)
	close(s.stopRecovery)

	// Cancel worker context to abort in-flight rclone operations
	s.workerCancel()

	// Wait for WaitGroup, BOUNDED by ctx
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
	}
}
