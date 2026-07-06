package cloudsync

import (
	"context"
	"fmt"

	"shakedown/internal/config"
)

// ProbeConfig carries the subset of config.Config fields the readiness probe
// needs. Kept separate from config.Config to avoid this package depending on
// the full config struct (and to make Probe trivially testable with fakes).
type ProbeConfig struct {
	Enabled      bool
	Remote       string
	PathTemplate string
}

// Probe reports whether cloud sync is currently usable and, if not, why.
// Checks run in order and short-circuit on the first failure:
//  1. feature flag disabled
//  2. no remote configured
//  3. path template invalid
//  4. rclone binary not runnable
//  5. configured remote not reachable/known to rclone
//
// Probe never panics: client is only ever dereferenced after cfg.Enabled and
// cfg.Remote have both passed, so a nil client (e.g. when cloud sync is
// disabled and no RemoteClient was constructed) is safe. It performs live
// rclone invocations (steps 4-5) on every call by design — this is an
// admin-only, infrequently-hit endpoint, and a fresh check is required so a
// remote fixed via POST /remote is reflected on the very next GET /status.
func Probe(ctx context.Context, cfg ProbeConfig, client RemoteClient) (bool, string) {
	if !cfg.Enabled {
		return false, "CLOUD_SYNC_ENABLED is false"
	}
	if cfg.Remote == "" {
		return false, "CLOUD_SYNC_REMOTE not set"
	}
	if err := config.ValidateTemplate(cfg.PathTemplate); err != nil {
		return false, err.Error()
	}
	if client == nil {
		return false, "rclone not found"
	}
	if _, err := client.Version(ctx); err != nil {
		return false, "rclone not found"
	}
	exists, err := client.RemoteExists(ctx)
	if err != nil || !exists {
		return false, fmt.Sprintf("remote '%s' not configured", cfg.Remote)
	}
	return true, ""
}
