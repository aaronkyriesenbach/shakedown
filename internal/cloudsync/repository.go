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
	CountByStatus(ctx context.Context) (map[string]int, error)
	ListFailedSyncs(ctx context.Context) ([]FailedSync, error)
}

// maxFailedSyncRows is the fixed safety-valve cap applied to
// ListFailedSyncs. The Failed Syncs list is intentionally unpaginated (see
// CONTEXT.md), so this cap exists purely to bound worst-case response size.
const maxFailedSyncRows = 500

// FailedSync is one row of the Failed Syncs list: a cloud_sync_state row
// that is either a Failed Sync proper (status='error') or mid-retry
// (status='syncing' AND error_class IS NOT NULL), joined with the owning
// recording's title. See CONTEXT.md for the Failed Sync / Error Class /
// Retry Status glossary.
type FailedSync struct {
	RecordingID   string
	Title         string
	Status        string
	Error         *string
	ErrorClass    *string
	Attempts      int
	LastAttemptAt *time.Time
	NextAttemptAt *time.Time
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
		WHERE recording_id=$1 AND lease_owner=$2 AND status='syncing'
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
		WHERE recording_id=$1 AND lease_owner=$2 AND status='syncing'
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
		WHERE recording_id=$1 AND lease_owner=$2 AND status='syncing'
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

// ListFailedSyncs returns Failed Syncs (status='error') plus mid-retry rows
// (status='syncing' AND error_class IS NOT NULL), joined with recordings to
// surface the title and exclude soft-deleted recordings. Ordered by
// last_attempt_at DESC and capped at maxFailedSyncRows.
func (s *PostgresStateStore) ListFailedSyncs(ctx context.Context) ([]FailedSync, error) {
	query := `
		SELECT cs.recording_id, r.title, cs.status, cs.error, cs.error_class,
		       cs.attempts, cs.last_attempt_at, cs.next_attempt_at
		FROM cloud_sync_state cs
		JOIN recordings r ON r.id = cs.recording_id
		WHERE r.deleted_at IS NULL
		  AND (cs.status = 'error' OR (cs.status = 'syncing' AND cs.error_class IS NOT NULL))
		ORDER BY cs.last_attempt_at DESC
		LIMIT $1
	`
	rows, err := s.db.Query(ctx, query, maxFailedSyncRows)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]FailedSync, 0)
	for rows.Next() {
		var row FailedSync
		if err := rows.Scan(
			&row.RecordingID,
			&row.Title,
			&row.Status,
			&row.Error,
			&row.ErrorClass,
			&row.Attempts,
			&row.LastAttemptAt,
			&row.NextAttemptAt,
		); err != nil {
			return nil, err
		}
		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

func (s *PostgresStateStore) CountByStatus(ctx context.Context) (map[string]int, error) {
	rows, err := s.db.Query(ctx, `SELECT status, COUNT(*) FROM cloud_sync_state GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		counts[status] = count
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return counts, nil
}
