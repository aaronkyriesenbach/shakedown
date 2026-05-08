package recordings

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Service wires together the repository, storage, and processing pipeline.
type Service struct {
	repo       *Repository
	storage    *LocalStorage
	logger     *zap.Logger
	maxWorkers int
	sem        chan struct{}
	wg         sync.WaitGroup
}

// NewService creates a new recordings Service with a bounded processing pool.
func NewService(repo *Repository, storage *LocalStorage, logger *zap.Logger, maxWorkers int) *Service {
	if maxWorkers <= 0 {
		maxWorkers = 4
	}
	return &Service{
		repo:       repo,
		storage:    storage,
		logger:     logger,
		maxWorkers: maxWorkers,
		sem:        make(chan struct{}, maxWorkers),
	}
}

// Enqueue submits a processing job to the background goroutine pool.
// It returns immediately; processing happens asynchronously.
func (svc *Service) Enqueue(job ProcessingJob, timeoutSeconds int) {
	svc.wg.Add(1)
	go func() {
		defer svc.wg.Done()

		svc.sem <- struct{}{}
		defer func() { <-svc.sem }()

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

// Shutdown waits for all in-flight processing jobs to complete.
func (svc *Service) Shutdown() {
	svc.wg.Wait()
}
