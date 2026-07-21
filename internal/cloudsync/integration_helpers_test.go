//go:build integration

package cloudsync

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"shakedown/internal/database"
)

const defaultTestDatabaseURL = "postgres://shakedown:shakedown@localhost:5432/shakedown?sslmode=disable"

// setupTestDB connects to the integration test database (running the
// embedded golang-migrate migrations UP via database.Connect, the single
// source of truth for migration logic) and returns the pool along with a
// cleanup func that truncates cloud_sync_state and closes the pool.
func setupTestDB(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()

	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		databaseURL = defaultTestDatabaseURL
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := database.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("setupTestDB: database.Connect failed: %v", err)
	}

	cleanup := func() {
		truncateCtx, truncateCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer truncateCancel()

		if _, err := pool.Exec(truncateCtx, "TRUNCATE cloud_sync_state"); err != nil {
			t.Errorf("setupTestDB: cleanup: failed to truncate cloud_sync_state: %v", err)
		}

		pool.Close()
	}

	return pool, cleanup
}

// TestHarness proves setupTestDB itself works: it connects (running
// migrations) and pings the resulting pool.
func TestHarness(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("TestHarness: ping failed: %v", err)
	}
}
