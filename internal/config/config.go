package config

import (
	"fmt"

	"github.com/kelseyhightower/envconfig"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	Port int `envconfig:"PORT" default:"8080"`

	DatabaseURL string `envconfig:"DATABASE_URL" required:"true"`

	StorageRoot string `envconfig:"STORAGE_ROOT" default:"/data"`

	DisableAuth      bool   `envconfig:"DISABLE_AUTH" default:"false"`
	OIDCIssuer       string `envconfig:"OIDC_ISSUER"`
	OIDCClientID     string `envconfig:"OIDC_CLIENT_ID"`
	OIDCClientSecret string `envconfig:"OIDC_CLIENT_SECRET"`
	AppBaseURL       string `envconfig:"APP_BASE_URL"`
	AdminGroup       string `envconfig:"ADMIN_GROUP"`

	SessionSecret string `envconfig:"SESSION_SECRET" required:"true"`

	ProcessingMaxWorkers     int `envconfig:"PROCESSING_MAX_WORKERS" default:"4"`
	ProcessingTimeoutSeconds int `envconfig:"PROCESSING_TIMEOUT_SECONDS" default:"1800"`

	UploadMaxSizeMB int64 `envconfig:"UPLOAD_MAX_SIZE_MB" default:"500"`

	// Video-specific configuration
	VideoProcessingMaxWorkers     int   `envconfig:"VIDEO_PROCESSING_MAX_WORKERS" default:"2"`
	VideoProcessingTimeoutSeconds int   `envconfig:"VIDEO_PROCESSING_TIMEOUT_SECONDS" default:"3600"`
	VideoUploadMaxSizeMB          int64 `envconfig:"VIDEO_UPLOAD_MAX_SIZE_MB" default:"4096"`

	// Recovery loop configuration
	RecoveryScanIntervalSeconds  int `envconfig:"RECOVERY_SCAN_INTERVAL_SECONDS" default:"120"`
	RecoveryStaleThresholdSeconds int `envconfig:"RECOVERY_STALE_THRESHOLD_SECONDS" default:"3900"`
}

func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("config: failed to load from environment: %w", err)
	}
	if !cfg.DisableAuth {
		for _, kv := range []struct{ name, val string }{
			{"OIDC_ISSUER", cfg.OIDCIssuer},
			{"OIDC_CLIENT_ID", cfg.OIDCClientID},
			{"OIDC_CLIENT_SECRET", cfg.OIDCClientSecret},
			{"APP_BASE_URL", cfg.AppBaseURL},
		} {
			if kv.val == "" {
				return nil, fmt.Errorf("config: %s is required when DISABLE_AUTH is not set", kv.name)
			}
		}
	}
	return &cfg, nil
}
