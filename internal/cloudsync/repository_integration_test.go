//go:build integration

package cloudsync

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"shakedown/internal/recordings"
)

func setupUserAndRecording(t *testing.T, db *pgxpool.Pool) (string, string) {
	ctx := context.Background()

	// Create user
	userID := uuid.NewString()
	_, err := db.Exec(ctx, "INSERT INTO users (id, oidc_sub, email, display_name) VALUES ($1, $2, $3, 'Test User')", userID, userID, userID+"@test.com")
	if err != nil {
		t.Fatalf("failed to insert user: %v", err)
	}

	// Create recording
	recRepo := recordings.NewRepository(db)
	rec, err := recRepo.Create(ctx, recordings.CreateRecordingInput{
		Title:            "Test Recording " + uuid.NewString(),
		FileExt:          "mp3",
		FileSizeBytes:    1024,
		MimeType:         "audio/mpeg",
		StoragePath:      "test/" + uuid.NewString() + ".mp3",
		UploadedBy:       userID,
		RecordedAt:       time.Now(),
		RecordedAtSource: "upload_timestamp",
		MediaType:        "audio",
	})
	if err != nil {
		t.Fatalf("failed to create recording: %v", err)
	}

	return userID, rec.ID
}

func TestPostgresStateStore_ClaimLeaseCollision(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	store := NewPostgresStateStore(db)

	t.Run("ClaimNew_Success_And_ConcurrentDuplicate", func(t *testing.T) {
		db.Exec(ctx, "TRUNCATE cloud_sync_state, recordings CASCADE")
		_, recID := setupUserAndRecording(t, db)

		owner1 := "owner1"
		owner2 := "owner2"

		// Claim 1
		res1, err := store.ClaimNew(ctx, recID, "path1.mp3", owner1, time.Minute, 5)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !res1.Claimed {
			t.Fatal("expected to claim successfully")
		}

		// Claim 2 (concurrent simulation via where clause)
		res2, err := store.ClaimNew(ctx, recID, "path2.mp3", owner2, time.Minute, 5)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res2.Claimed {
			t.Fatal("expected second claim to fail because lease is active")
		}
	})

	t.Run("ClaimNew_AlreadySynced", func(t *testing.T) {
		db.Exec(ctx, "TRUNCATE cloud_sync_state, recordings CASCADE")
		_, recID := setupUserAndRecording(t, db)

		owner := "owner1"
		_, _ = store.ClaimNew(ctx, recID, "path1.mp3", owner, time.Minute, 5) // ignoring result

		_, err := store.MarkSynced(ctx, recID, owner, "remote_id_1", 1024)
		if err != nil {
			t.Fatalf("marksynced failed: %v", err)
		}

		res2, err := store.ClaimNew(ctx, recID, "path2.mp3", owner, time.Minute, 5)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res2.Claimed {
			t.Fatal("expected synced row to not be claimed")
		}
	})

	t.Run("ClaimNew_ExpiredLease", func(t *testing.T) {
		db.Exec(ctx, "TRUNCATE cloud_sync_state, recordings CASCADE")
		_, recID := setupUserAndRecording(t, db)

		owner1 := "owner1"
		owner2 := "owner2"

		// Claim with short/negative TTL
		res1, _ := store.ClaimNew(ctx, recID, "path1.mp3", owner1, -time.Minute, 5)
		if !res1.Claimed {
			t.Fatal("expected initial claim to succeed")
		}

		// Re-claim by owner2 should succeed since lease expired
		res2, err := store.ClaimNew(ctx, recID, "path2.mp3", owner2, time.Minute, 5)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !res2.Claimed {
			t.Fatal("expected expired lease to be re-claimable")
		}
		if res2.RemotePath != "path1.mp3" {
			t.Fatalf("expected re-claim to return original RemotePath 'path1.mp3', got '%s'", res2.RemotePath)
		}
	})

	t.Run("ClaimNew_MaxAttempts", func(t *testing.T) {
		db.Exec(ctx, "TRUNCATE cloud_sync_state, recordings CASCADE")
		_, recID := setupUserAndRecording(t, db)

		owner := "owner1"
		res1, _ := store.ClaimNew(ctx, recID, "path1.mp3", owner, -time.Minute, 1) // attempt 1
		if !res1.Claimed {
			t.Fatal("expected claim 1 to succeed")
		}

		res2, err := store.ClaimNew(ctx, recID, "path1.mp3", owner, time.Minute, 1) // max attempts = 1
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res2.Claimed {
			t.Fatal("expected claim to fail because attempts >= max")
		}
	})

	t.Run("ClaimNew_Collision", func(t *testing.T) {
		db.Exec(ctx, "TRUNCATE cloud_sync_state, recordings CASCADE")
		_, recID1 := setupUserAndRecording(t, db)
		_, recID2 := setupUserAndRecording(t, db)

		owner := "owner1"

		res1, err := store.ClaimNew(ctx, recID1, "duplicate.mp3", owner, time.Minute, 5)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !res1.Claimed {
			t.Fatal("expected claim to succeed")
		}

		_, err = store.ClaimNew(ctx, recID2, "duplicate.mp3", owner, time.Minute, 5)
		if err != ErrRemotePathConflict {
			t.Fatalf("expected ErrRemotePathConflict, got %v", err)
		}
	})

	t.Run("OwnerFencing_RejectsWrongOwner", func(t *testing.T) {
		db.Exec(ctx, "TRUNCATE cloud_sync_state, recordings CASCADE")
		_, recID := setupUserAndRecording(t, db)

		owner1 := "owner1"
		owner2 := "owner2"

		res1, _ := store.ClaimNew(ctx, recID, "path1.mp3", owner1, time.Minute, 5)
		if !res1.Claimed {
			t.Fatal("expected claim to succeed")
		}

		// Attempt to mark synced with owner2
		rows, err := store.MarkSynced(ctx, recID, owner2, "fake-id", 999)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rows != 0 {
			t.Fatal("expected 0 rows affected for different owner")
		}

		// Attempt to mark error with owner2
		rows, err = store.MarkError(ctx, recID, owner2, "some_class", "some_msg", time.Now())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rows != 0 {
			t.Fatal("expected 0 rows affected for different owner")
		}

		// Ensure status remains 'syncing' and lease_owner is still owner1
		state, err := store.Get(ctx, recID)
		if err != nil {
			t.Fatalf("unexpected error getting state: %v", err)
		}
		if state.Status != "syncing" || *state.LeaseOwner != owner1 {
			t.Fatalf("expected status 'syncing' and owner '%s', got status='%s'", owner1, state.Status)
		}

		// Attempt to mark synced with owner1
		rows, err = store.MarkSynced(ctx, recID, owner1, "real-id", 12345)
		if err != nil || rows != 1 {
			t.Fatalf("expected real owner to mark synced. err: %v, rows: %d", err, rows)
		}
	})

	t.Run("TerminalOps_StatusFencing", func(t *testing.T) {
		db.Exec(ctx, "TRUNCATE cloud_sync_state, recordings CASCADE")
		_, recID := setupUserAndRecording(t, db)

		owner := "owner1"
		res1, _ := store.ClaimNew(ctx, recID, "path1.mp3", owner, time.Minute, 5)
		if !res1.Claimed {
			t.Fatal("expected claim to succeed")
		}

		rows, err := store.MarkSynced(ctx, recID, owner, "real-id", 100)
		if err != nil || rows != 1 {
			t.Fatalf("expected mark synced to succeed: err=%v, rows=%d", err, rows)
		}

		// Try to MarkSynced again with the same owner (row is now 'synced')
		rows, err = store.MarkSynced(ctx, recID, owner, "real-id-2", 200)
		if err != nil || rows != 0 {
			t.Fatalf("expected mark synced to fail on already synced row: err=%v, rows=%d", err, rows)
		}
	})
}

func TestPostgresStateStore_ClaimRetry(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	store := NewPostgresStateStore(db)

	t.Run("BypassesMaxAttemptsGate_ReclaimsExhaustedRow", func(t *testing.T) {
		db.Exec(ctx, "TRUNCATE cloud_sync_state, recordings CASCADE")
		_, recID := setupUserAndRecording(t, db)

		owner := "owner1"
		// Claim and exhaust: attempts reaches max_attempts (1), then mark error.
		res, err := store.ClaimNew(ctx, recID, "path1.mp3", owner, time.Minute, 1)
		if err != nil || !res.Claimed {
			t.Fatalf("expected initial claim to succeed: err=%v claimed=%v", err, res.Claimed)
		}
		if _, err := store.MarkError(ctx, recID, owner, "copy_failed", "disk full", time.Now().Add(time.Hour)); err != nil {
			t.Fatalf("mark error failed: %v", err)
		}

		// A normal ClaimNew must NOT be able to reclaim this row: attempts (1) >= max_attempts (1).
		blocked, err := store.ClaimNew(ctx, recID, "path1.mp3", "owner2", time.Minute, 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if blocked.Claimed {
			t.Fatal("expected ClaimNew to respect the max_attempts gate and refuse to reclaim an Exhausted row")
		}

		// ClaimRetry bypasses the gate and reclaims the same Exhausted row,
		// reusing its existing remote_path and incrementing (not resetting) attempts.
		res2, err := store.ClaimRetry(ctx, recID, "owner3", time.Minute)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !res2.Claimed {
			t.Fatal("expected ClaimRetry to bypass the max_attempts gate and reclaim the Exhausted row")
		}
		if res2.RemotePath != "path1.mp3" {
			t.Fatalf("expected ClaimRetry to reuse the existing remote_path, got %q", res2.RemotePath)
		}
		if res2.Attempts != 2 {
			t.Fatalf("expected attempts to increment from 1 to 2 (not reset), got %d", res2.Attempts)
		}

		state, err := store.Get(ctx, recID)
		if err != nil {
			t.Fatalf("unexpected error getting state: %v", err)
		}
		if state.Status != "syncing" {
			t.Fatalf("expected status='syncing' after ClaimRetry, got %q", state.Status)
		}
		if state.LeaseOwner == nil || *state.LeaseOwner != "owner3" {
			t.Fatalf("expected lease_owner='owner3', got %v", state.LeaseOwner)
		}
	})

	t.Run("RefusesToStealALiveLease", func(t *testing.T) {
		db.Exec(ctx, "TRUNCATE cloud_sync_state, recordings CASCADE")
		_, recID := setupUserAndRecording(t, db)

		owner := "owner1"
		res, err := store.ClaimNew(ctx, recID, "path1.mp3", owner, time.Minute, 5)
		if err != nil || !res.Claimed {
			t.Fatalf("expected initial claim to succeed: err=%v claimed=%v", err, res.Claimed)
		}

		// The row is mid-retry (status='syncing') under a live (not yet expired) lease.
		res2, err := store.ClaimRetry(ctx, recID, "owner2", time.Minute)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res2.Claimed {
			t.Fatal("expected ClaimRetry to refuse to steal a live in-flight lease")
		}

		state, err := store.Get(ctx, recID)
		if err != nil {
			t.Fatalf("unexpected error getting state: %v", err)
		}
		if state.LeaseOwner == nil || *state.LeaseOwner != owner {
			t.Fatalf("expected original owner to remain, got %v", state.LeaseOwner)
		}
		if state.Attempts != 1 {
			t.Fatalf("expected attempts to remain unchanged at 1, got %d", state.Attempts)
		}
	})

	t.Run("ReclaimsExpiredMidRetryLease", func(t *testing.T) {
		db.Exec(ctx, "TRUNCATE cloud_sync_state, recordings CASCADE")
		_, recID := setupUserAndRecording(t, db)

		owner := "owner1"
		res, err := store.ClaimNew(ctx, recID, "path1.mp3", owner, -time.Minute, 5) // negative TTL -> already expired
		if err != nil || !res.Claimed {
			t.Fatalf("expected initial claim to succeed: err=%v claimed=%v", err, res.Claimed)
		}

		res2, err := store.ClaimRetry(ctx, recID, "owner2", time.Minute)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !res2.Claimed {
			t.Fatal("expected ClaimRetry to reclaim a mid-retry row whose lease has expired")
		}
		if res2.Attempts != 2 {
			t.Fatalf("expected attempts to increment from 1 to 2, got %d", res2.Attempts)
		}
	})

	t.Run("NoRowReturnsNotClaimedWithoutError", func(t *testing.T) {
		db.Exec(ctx, "TRUNCATE cloud_sync_state, recordings CASCADE")

		res, err := store.ClaimRetry(ctx, uuid.NewString(), "owner1", time.Minute)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Claimed {
			t.Fatal("expected Claimed=false for a recording_id with no cloud_sync_state row")
		}
	})

	t.Run("RefusesToClaimASyncedRow", func(t *testing.T) {
		db.Exec(ctx, "TRUNCATE cloud_sync_state, recordings CASCADE")
		_, recID := setupUserAndRecording(t, db)

		owner := "owner1"
		res, err := store.ClaimNew(ctx, recID, "path1.mp3", owner, time.Minute, 5)
		if err != nil || !res.Claimed {
			t.Fatalf("expected initial claim to succeed: err=%v claimed=%v", err, res.Claimed)
		}
		if _, err := store.MarkSynced(ctx, recID, owner, "remote-id", 100); err != nil {
			t.Fatalf("mark synced failed: %v", err)
		}

		res2, err := store.ClaimRetry(ctx, recID, "owner2", time.Minute)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res2.Claimed {
			t.Fatal("expected ClaimRetry to refuse to reclaim an already-synced row")
		}
	})
}

func TestPostgresStateStore_ListFailedSyncs(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	store := NewPostgresStateStore(db)
	recRepo := recordings.NewRepository(db)

	t.Run("JoinsTitle_ExcludesSoftDeleted_IncludesMidRetry_OrdersByLastAttemptDesc", func(t *testing.T) {
		db.Exec(ctx, "TRUNCATE cloud_sync_state, recordings CASCADE")

		// Row 1: a genuine Failed Sync (status='error'), oldest last_attempt_at.
		_, recErr := setupUserAndRecording(t, db)
		_, err := store.ClaimNew(ctx, recErr, "path-err.mp3", "owner1", time.Minute, 5)
		if err != nil {
			t.Fatalf("claim failed: %v", err)
		}
		if _, err := store.MarkError(ctx, recErr, "owner1", "copy_failed", "disk full", time.Now().Add(time.Hour)); err != nil {
			t.Fatalf("mark error failed: %v", err)
		}
		if _, err := db.Exec(ctx, "UPDATE cloud_sync_state SET last_attempt_at = now() - interval '2 hours' WHERE recording_id = $1", recErr); err != nil {
			t.Fatalf("failed to backdate last_attempt_at: %v", err)
		}

		// Row 2: mid-retry (status='syncing' AND error_class IS NOT NULL),
		// most recent last_attempt_at -> must sort first.
		_, recRetrying := setupUserAndRecording(t, db)
		if _, err := store.ClaimNew(ctx, recRetrying, "path-retry.mp3", "owner1", time.Minute, 5); err != nil {
			t.Fatalf("claim failed: %v", err)
		}
		if _, err := db.Exec(ctx, "UPDATE cloud_sync_state SET error_class = 'copy_failed', error = 'retrying now' WHERE recording_id = $1", recRetrying); err != nil {
			t.Fatalf("failed to set error_class on syncing row: %v", err)
		}

		// Row 3: soft-deleted recording with status='error' -> must be excluded.
		_, recDeleted := setupUserAndRecording(t, db)
		if _, err := store.ClaimNew(ctx, recDeleted, "path-deleted.mp3", "owner1", time.Minute, 5); err != nil {
			t.Fatalf("claim failed: %v", err)
		}
		if _, err := store.MarkError(ctx, recDeleted, "owner1", "copy_failed", "boom", time.Now().Add(time.Hour)); err != nil {
			t.Fatalf("mark error failed: %v", err)
		}
		if err := recRepo.SoftDelete(ctx, recDeleted); err != nil {
			t.Fatalf("soft delete failed: %v", err)
		}

		// Row 4: pending, not yet attempted -> not a Failed Sync, must be excluded.
		_, recPending := setupUserAndRecording(t, db)
		if _, err := db.Exec(ctx, `INSERT INTO cloud_sync_state (recording_id, remote_path, status) VALUES ($1, 'path-pending.mp3', 'pending')`, recPending); err != nil {
			t.Fatalf("failed to insert pending row: %v", err)
		}

		// Row 5: synced -> must be excluded.
		_, recSynced := setupUserAndRecording(t, db)
		if _, err := store.ClaimNew(ctx, recSynced, "path-synced.mp3", "owner1", time.Minute, 5); err != nil {
			t.Fatalf("claim failed: %v", err)
		}
		if _, err := store.MarkSynced(ctx, recSynced, "owner1", "remote-id", 100); err != nil {
			t.Fatalf("mark synced failed: %v", err)
		}

		rows, err := store.ListFailedSyncs(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(rows) != 2 {
			t.Fatalf("expected 2 rows (error + mid-retry, excluding soft-deleted/pending/synced), got %d: %+v", len(rows), rows)
		}

		// Ordered by last_attempt_at DESC: the mid-retry row is most recent.
		if rows[0].RecordingID != recRetrying {
			t.Fatalf("expected most recent row first (recRetrying), got %s", rows[0].RecordingID)
		}
		if rows[0].Status != "syncing" {
			t.Fatalf("expected mid-retry row status='syncing', got %q", rows[0].Status)
		}
		if rows[0].ErrorClass == nil || *rows[0].ErrorClass != "copy_failed" {
			t.Fatalf("expected mid-retry row error_class='copy_failed', got %v", rows[0].ErrorClass)
		}

		if rows[1].RecordingID != recErr {
			t.Fatalf("expected second row to be recErr, got %s", rows[1].RecordingID)
		}
		if rows[1].Status != "error" {
			t.Fatalf("expected recErr row status='error', got %q", rows[1].Status)
		}
		if rows[1].Title == "" {
			t.Fatalf("expected title to be joined from recordings, got empty string")
		}
		if rows[1].Error == nil || *rows[1].Error != "disk full" {
			t.Fatalf("expected error='disk full', got %v", rows[1].Error)
		}
		if rows[1].Attempts != 1 {
			t.Fatalf("expected attempts=1, got %d", rows[1].Attempts)
		}
		if rows[1].LastAttemptAt == nil {
			t.Fatalf("expected last_attempt_at to be set")
		}
		if rows[1].NextAttemptAt == nil {
			t.Fatalf("expected next_attempt_at to be set")
		}
	})

	t.Run("CapsAt500", func(t *testing.T) {
		db.Exec(ctx, "TRUNCATE cloud_sync_state, recordings CASCADE")

		const total = maxFailedSyncRows + 10
		for i := 0; i < total; i++ {
			_, recID := setupUserAndRecording(t, db)
			if _, err := store.ClaimNew(ctx, recID, "path-"+recID+".mp3", "owner1", time.Minute, 5); err != nil {
				t.Fatalf("claim failed: %v", err)
			}
			if _, err := store.MarkError(ctx, recID, "owner1", "copy_failed", "boom", time.Now().Add(time.Hour)); err != nil {
				t.Fatalf("mark error failed: %v", err)
			}
		}

		rows, err := store.ListFailedSyncs(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(rows) != maxFailedSyncRows {
			t.Fatalf("expected cap of %d rows, got %d", maxFailedSyncRows, len(rows))
		}
	})
}

func TestListAllForSync(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	db.Exec(ctx, "TRUNCATE cloud_sync_state, recordings CASCADE")
	recRepo := recordings.NewRepository(db)

	_, recID1 := setupUserAndRecording(t, db) // valid
	_, recID2 := setupUserAndRecording(t, db) // will soft-delete
	_, recID3 := setupUserAndRecording(t, db) // empty storage path

	// Soft delete rec 2
	if err := recRepo.SoftDelete(ctx, recID2); err != nil {
		t.Fatalf("failed to soft delete: %v", err)
	}

	// Update rec 3 to have empty storage_path
	_, err := db.Exec(ctx, "UPDATE recordings SET storage_path = '' WHERE id = $1", recID3)
	if err != nil {
		t.Fatalf("failed to update storage path: %v", err)
	}

	candidates, err := recRepo.ListAllForSync(ctx, "", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}

	if candidates[0].ID != recID1 {
		t.Fatalf("expected rec1, got %s", candidates[0].ID)
	}
}
