package config

import "testing"

// setRequiredEnv sets the minimum environment variables needed for Load() to
// succeed (DATABASE_URL, SESSION_SECRET, and DISABLE_AUTH=true so the OIDC
// fields aren't required), using t.Setenv so they're cleaned up automatically.
func setRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("DISABLE_AUTH", "true")
}

func TestLoadCloudSyncDefaults(t *testing.T) {
	setRequiredEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}

	if cfg.CloudSyncEnabled != false {
		t.Errorf("CloudSyncEnabled default: got %v, want false", cfg.CloudSyncEnabled)
	}
	if cfg.CloudSyncRcloneBin != "rclone" {
		t.Errorf("CloudSyncRcloneBin default: got %q, want %q", cfg.CloudSyncRcloneBin, "rclone")
	}
	if cfg.RcloneConfigPath != "/data/rclone/rclone.conf" {
		t.Errorf("RcloneConfigPath default: got %q, want %q", cfg.RcloneConfigPath, "/data/rclone/rclone.conf")
	}
	if cfg.CloudSyncRemote != "" {
		t.Errorf("CloudSyncRemote default: got %q, want empty", cfg.CloudSyncRemote)
	}
	if cfg.CloudSyncRoot != "Shakedown" {
		t.Errorf("CloudSyncRoot default: got %q, want %q", cfg.CloudSyncRoot, "Shakedown")
	}
	if cfg.CloudSyncPathTemplate != "{year}/{date}/{title}.{ext}" {
		t.Errorf("CloudSyncPathTemplate default: got %q, want %q", cfg.CloudSyncPathTemplate, "{year}/{date}/{title}.{ext}")
	}
	if cfg.CloudSyncIntervalSeconds != 3600 {
		t.Errorf("CloudSyncIntervalSeconds default: got %d, want 3600", cfg.CloudSyncIntervalSeconds)
	}
	if cfg.CloudSyncMaxWorkers != 2 {
		t.Errorf("CloudSyncMaxWorkers default: got %d, want 2", cfg.CloudSyncMaxWorkers)
	}
	if cfg.CloudSyncMaxAttempts != 5 {
		t.Errorf("CloudSyncMaxAttempts default: got %d, want 5", cfg.CloudSyncMaxAttempts)
	}
	if cfg.CloudSyncLeaseTTLSeconds != 900 {
		t.Errorf("CloudSyncLeaseTTLSeconds default: got %d, want 900", cfg.CloudSyncLeaseTTLSeconds)
	}
	if cfg.CloudSyncBackoffBaseSeconds != 60 {
		t.Errorf("CloudSyncBackoffBaseSeconds default: got %d, want 60", cfg.CloudSyncBackoffBaseSeconds)
	}
	if cfg.CloudSyncTPSLimit != 0 {
		t.Errorf("CloudSyncTPSLimit default: got %d, want 0", cfg.CloudSyncTPSLimit)
	}
}

func TestLoadCloudSyncEnabledWithoutRemoteDoesNotError(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("CLOUD_SYNC_ENABLED", "true")
	// CLOUD_SYNC_REMOTE intentionally left unset (empty).

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() must not error when cloud sync is enabled with no remote configured: %v", err)
	}
	if !cfg.CloudSyncEnabled {
		t.Error("expected CloudSyncEnabled to be true")
	}
	if cfg.CloudSyncRemote != "" {
		t.Errorf("expected CloudSyncRemote empty, got %q", cfg.CloudSyncRemote)
	}
}

func TestLoadDoesNotValidateTemplate(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("CLOUD_SYNC_PATH_TEMPLATE", "{bogus}")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() must never validate the template, got error: %v", err)
	}
	if cfg.CloudSyncPathTemplate != "{bogus}" {
		t.Errorf("CloudSyncPathTemplate: got %q, want %q", cfg.CloudSyncPathTemplate, "{bogus}")
	}
	// Confirm the invalid template is in fact something ValidateTemplate
	// would reject later, at probe time (Todo 11) - not here.
	if err := ValidateTemplate(cfg.CloudSyncPathTemplate); err == nil {
		t.Error("expected ValidateTemplate to reject {bogus}, this test's premise depends on it")
	}
}

func TestLoadExistingAuthBehaviorUnchanged(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("DISABLE_AUTH", "false")
	// OIDC_ISSUER, OIDC_CLIENT_ID, OIDC_CLIENT_SECRET, APP_BASE_URL left unset.

	if _, err := Load(); err == nil {
		t.Fatal("expected Load() to still error when DISABLE_AUTH=false and OIDC fields are missing")
	}
}
