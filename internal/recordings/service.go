package recordings

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Service wires together the repository, storage, and processing pipeline.
type Service struct {
	repo         *Repository
	storage      *LocalStorage
	logger       *zap.Logger
	audioSem     chan struct{}
	videoSem     chan struct{}
	wg           sync.WaitGroup
	stopRecovery chan struct{}
}

// NewService creates a new recordings Service with a bounded processing pool.
func NewService(repo *Repository, storage *LocalStorage, logger *zap.Logger, maxAudioWorkers, maxVideoWorkers int) *Service {
	if maxAudioWorkers <= 0 {
		maxAudioWorkers = 4
	}
	if maxVideoWorkers <= 0 {
		maxVideoWorkers = 2
	}
	return &Service{
		repo:         repo,
		storage:      storage,
		logger:       logger,
		audioSem:     make(chan struct{}, maxAudioWorkers),
		videoSem:     make(chan struct{}, maxVideoWorkers),
		stopRecovery: make(chan struct{}),
	}
}

// Enqueue submits a processing job to the background goroutine pool.
// It returns immediately; processing happens asynchronously.
func (svc *Service) Enqueue(job ProcessingJob, timeoutSeconds int) {
	svc.wg.Add(1)
	go func() {
		defer svc.wg.Done()

		// Select semaphore based on media type
		var sem chan struct{}
		if job.MediaType == "video" {
			sem = svc.videoSem
		} else {
			sem = svc.audioSem
		}

		sem <- struct{}{}
		defer func() { <-sem }()

		timeout := time.Duration(timeoutSeconds) * time.Second
		if timeout <= 0 {
			timeout = 5 * time.Minute
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		svc.logger.Info("processing started", zap.String("recording_id", job.RecordingID))
		svc.processRecording(ctx, job)
		svc.logger.Info("processing finished", zap.String("recording_id", job.RecordingID))
	}()
}

// StartRecoveryLoop launches a background goroutine that periodically scans for
// recordings stuck in a non-terminal processing step and re-enqueues them.
// A recording is considered stuck when its updated_at is older than
// staleThreshold. The scan runs every scanInterval.
func (svc *Service) StartRecoveryLoop(scanInterval, staleThreshold time.Duration, audioTimeout, videoTimeout int) {
	svc.wg.Add(1)
	go func() {
		defer svc.wg.Done()

		svc.recoverStuckJobs(staleThreshold, audioTimeout, videoTimeout)

		ticker := time.NewTicker(scanInterval)
		defer ticker.Stop()

		for {
			select {
			case <-svc.stopRecovery:
				return
			case <-ticker.C:
				svc.recoverStuckJobs(staleThreshold, audioTimeout, videoTimeout)
			}
		}
	}()
}

func (svc *Service) recoverStuckJobs(staleThreshold time.Duration, audioTimeout, videoTimeout int) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stuck, err := svc.repo.FindStuckRecordings(ctx, staleThreshold)
	if err != nil {
		svc.logger.Error("recovery: failed to query stuck recordings", zap.Error(err))
		return
	}

	for _, rec := range stuck {
		svc.logger.Info("recovery: re-enqueuing stuck recording",
			zap.String("recording_id", rec.ID),
			zap.String("stuck_step", rec.ProcessingStep),
			zap.Time("last_updated", rec.UpdatedAt),
		)

		if err := svc.repo.UpdateProcessingStep(ctx, rec.ID, "queued"); err != nil {
			svc.logger.Error("recovery: failed to reset processing step",
				zap.String("recording_id", rec.ID),
				zap.Error(err),
			)
			continue
		}

		timeout := audioTimeout
		if rec.MediaType == "video" {
			timeout = videoTimeout
		}

		svc.Enqueue(ProcessingJob{
			RecordingID: rec.ID,
			StorageRoot: svc.storage.root,
			FileExt:     rec.FileExt,
			MediaType:   rec.MediaType,
		}, timeout)
	}

	if len(stuck) > 0 {
		svc.logger.Info("recovery: re-enqueued stuck recordings", zap.Int("count", len(stuck)))
	}
}

// Shutdown stops the recovery loop and waits for all in-flight processing jobs to complete.
func (svc *Service) Shutdown() {
	close(svc.stopRecovery)
	svc.wg.Wait()
}
