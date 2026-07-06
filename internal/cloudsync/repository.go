package cloudsync

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrRemotePathConflict = errors.New("cloudsync: remote path already claimed by another recording")

type ClaimResult struct {
	Claimed    bool
	RemotePath string
	Attempts   int
}

type SyncState struct {
	RecordingID     string
	RemotePath      string
	RemoteFileID    *string
	FileSizeBytes   *int64
	RemoteSizeBytes *int64
	Status          string
	Error           *string
	ErrorClass      *string
	Attempts        int
	LastAttemptAt   *time.Time
	NextAttemptAt   *time.Time
	LeaseOwner      *string
	LeaseExpiresAt  *time.Time
	SyncedAt        *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type StateStore interface {
	ClaimNew(ctx context.Context, recordingID, remotePath, owner string, leaseTTL time.Duration, maxAttempts int) (ClaimResult, error)
	Get(ctx context.Context, recordingID string) (*SyncState, error)
	MarkSynced(ctx context.Context, recordingID, owner, remoteFileID string, remoteSize int64) (int64, error)
	MarkError(ctx context.Context, recordingID, owner, errClass, errMsg string, nextAttemptAt time.Time) (int64, error)
	Heartbeat(ctx context.Context, recordingID, owner string, ttl time.Duration) (rowsAffected int64, err error)
	RecoverExpiredLeases(ctx context.Context) (int64, error)
}

type PostgresStateStore struct {
	db *pgxpool.Pool
}

func NewPostgresStateStore(db *pgxpool.Pool) *PostgresStateStore {
	return &PostgresStateStore{db: db}
}

func (s *PostgresStateStore) ClaimNew(ctx context.Context, recordingID, remotePath, owner string, leaseTTL time.Duration, maxAttempts int) (ClaimResult, error) {
	query := `
		INSERT INTO cloud_sync_state (recording_id, remote_path, status, attempts, last_attempt_at, lease_owner, lease_expires_at)
		VALUES ($1, $2, 'syncing', 1, now(), $3, now()+$4::interval)
		ON CONFLICT (recording_id) DO UPDATE SET
			status='syncing', attempts=cloud_sync_state.attempts+1, last_attempt_at=now(),
			lease_owner=$3, lease_expires_at=now()+$4::interval
		WHERE (cloud_sync_state.status IN ('pending','error')
			OR (cloud_sync_state.status='syncing' AND cloud_sync_state.lease_expires_at < now()))
			AND cloud_sync_state.attempts < $5
			AND (cloud_sync_state.next_attempt_at IS NULL OR cloud_sync_state.next_attempt_at <= now())
		RETURNING remote_path, attempts
	`

	var retPath string
	var attempts int

	err := s.db.QueryRow(ctx, query, recordingID, remotePath, owner, leaseTTL, maxAttempts).Scan(&retPath, &attempts)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ClaimResult{Claimed: false}, nil
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "uq_cloud_sync_remote_path" {
			return ClaimResult{}, ErrRemotePathConflict
		}
		return ClaimResult{}, err
	}

	return ClaimResult{
		Claimed:    true,
		RemotePath: retPath,
		Attempts:   attempts,
	}, nil
}

func (s *PostgresStateStore) Get(ctx context.Context, recordingID string) (*SyncState, error) {
	query := `
		SELECT recording_id, remote_path, remote_file_id, file_size_bytes, remote_size_bytes,
		       status, error, error_class, attempts, last_attempt_at, next_attempt_at,
		       lease_owner, lease_expires_at, synced_at, created_at, updated_at
		FROM cloud_sync_state
		WHERE recording_id = $1
	`
	var state SyncState
	err := s.db.QueryRow(ctx, query, recordingID).Scan(
		&state.RecordingID,
		&state.RemotePath,
		&state.RemoteFileID,
		&state.FileSizeBytes,
		&state.RemoteSizeBytes,
		&state.Status,
		&state.Error,
		&state.ErrorClass,
		&state.Attempts,
		&state.LastAttemptAt,
		&state.NextAttemptAt,
		&state.LeaseOwner,
		&state.LeaseExpiresAt,
		&state.SyncedAt,
		&state.CreatedAt,
		&state.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // Or a specific ErrNotFound
		}
		return nil, err
	}
	return &state, nil
}

func (s *PostgresStateStore) MarkSynced(ctx context.Context, recordingID, owner, remoteFileID string, remoteSize int64) (int64, error) {
	query := `
		UPDATE cloud_sync_state
		SET status='synced', remote_file_id=$3, remote_size_bytes=$4, synced_at=now(), updated_at=now(),
		    lease_owner=NULL, lease_expires_at=NULL, error=NULL, error_class=NULL, next_attempt_at=NULL
		WHERE recording_id=$1 AND lease_owner=$2
	`
	tag, err := s.db.Exec(ctx, query, recordingID, owner, remoteFileID, remoteSize)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (s *PostgresStateStore) MarkError(ctx context.Context, recordingID, owner, errClass, errMsg string, nextAttemptAt time.Time) (int64, error) {
	query := `
		UPDATE cloud_sync_state
		SET status='error', error_class=$3, error=$4, next_attempt_at=$5, updated_at=now(),
		    lease_owner=NULL, lease_expires_at=NULL
		WHERE recording_id=$1 AND lease_owner=$2
	`
	tag, err := s.db.Exec(ctx, query, recordingID, owner, errClass, errMsg, nextAttemptAt)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (s *PostgresStateStore) Heartbeat(ctx context.Context, recordingID, owner string, ttl time.Duration) (int64, error) {
	query := `
		UPDATE cloud_sync_state
		SET lease_expires_at=now()+$3::interval, updated_at=now()
		WHERE recording_id=$1 AND lease_owner=$2
	`
	tag, err := s.db.Exec(ctx, query, recordingID, owner, ttl)
	return tag.RowsAffected(), err
}

func (s *PostgresStateStore) RecoverExpiredLeases(ctx context.Context) (int64, error) {
	query := `
		UPDATE cloud_sync_state
		SET status='error', error_class='lease_expired', error='lease expired', next_attempt_at=now(), updated_at=now(),
		    lease_owner=NULL, lease_expires_at=NULL
		WHERE status='syncing' AND lease_expires_at < now()
	`
	tag, err := s.db.Exec(ctx, query)
	return tag.RowsAffected(), err
}
