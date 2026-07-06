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
