package cloudsync

import (
	"context"
	"errors"
	"testing"
)

// fakeProbeClient is a configurable RemoteClient fake dedicated to Probe's
// tests — the shared fakeRemoteClient in service_test.go always succeeds and
// has no way to simulate Version/RemoteExists failures.
type fakeProbeClient struct {
	versionErr error
	existsOK   bool
	existsErr  error
}

func (f *fakeProbeClient) Version(ctx context.Context) (string, error) {
	if f.versionErr != nil {
		return "", f.versionErr
	}
	return "rclone v1.68.0", nil
}
func (f *fakeProbeClient) RemoteExists(ctx context.Context) (bool, error) {
	return f.existsOK, f.existsErr
}
func (f *fakeProbeClient) WriteRemoteConfig(ctx context.Context, block string) error { return nil }
func (f *fakeProbeClient) Copy(ctx context.Context, localAbsPath, remotePath string) error {
	return nil
}
func (f *fakeProbeClient) StatSize(ctx context.Context, remotePath string) (int64, bool, error) {
	return 0, false, nil
}

func TestProbe_Disabled(t *testing.T) {
	enabled, reason := Probe(context.Background(), ProbeConfig{Enabled: false}, nil)
	if enabled || reason != "CLOUD_SYNC_ENABLED is false" {
		t.Fatalf("got (%v, %q)", enabled, reason)
	}
}

func TestProbe_NilClient_NeverPanics(t *testing.T) {
	// Disabled short-circuits before ever touching the nil client.
	enabled, reason := Probe(context.Background(), ProbeConfig{Enabled: false, Remote: "", PathTemplate: "{bogus}"}, nil)
	if enabled || reason != "CLOUD_SYNC_ENABLED is false" {
		t.Fatalf("got (%v, %q)", enabled, reason)
	}
}

func TestProbe_NoRemote(t *testing.T) {
	enabled, reason := Probe(context.Background(), ProbeConfig{Enabled: true, Remote: ""}, nil)
	if enabled || reason != "CLOUD_SYNC_REMOTE not set" {
		t.Fatalf("got (%v, %q)", enabled, reason)
	}
}

func TestProbe_InvalidTemplate(t *testing.T) {
	enabled, reason := Probe(context.Background(), ProbeConfig{
		Enabled:      true,
		Remote:       "myremote",
		PathTemplate: "{bogus}",
	}, nil)
	if enabled {
		t.Fatalf("expected disabled, got enabled")
	}
	if reason != `unknown template token "{bogus}"` {
		t.Fatalf("unexpected reason: %q", reason)
	}
}

func TestProbe_RcloneNotFound(t *testing.T) {
	client := &fakeProbeClient{versionErr: errors.New("exec: \"rclone\": executable file not found in $PATH")}
	enabled, reason := Probe(context.Background(), ProbeConfig{
		Enabled:      true,
		Remote:       "myremote",
		PathTemplate: "{year}/{title}.{ext}",
	}, client)
	if enabled || reason != "rclone not found" {
		t.Fatalf("got (%v, %q)", enabled, reason)
	}
}

func TestProbe_NilClient_WhenEnabled(t *testing.T) {
	// Defensive: even if enabled+remote+template all pass, a nil client
	// (should never happen in practice once enabled, but defends against
	// wiring mistakes) must not panic.
	enabled, reason := Probe(context.Background(), ProbeConfig{
		Enabled:      true,
		Remote:       "myremote",
		PathTemplate: "{year}/{title}.{ext}",
	}, nil)
	if enabled || reason != "rclone not found" {
		t.Fatalf("got (%v, %q)", enabled, reason)
	}
}

func TestProbe_RemoteNotConfigured(t *testing.T) {
	client := &fakeProbeClient{existsOK: false}
	enabled, reason := Probe(context.Background(), ProbeConfig{
		Enabled:      true,
		Remote:       "myremote",
		PathTemplate: "{year}/{title}.{ext}",
	}, client)
	if enabled || reason != "remote 'myremote' not configured" {
		t.Fatalf("got (%v, %q)", enabled, reason)
	}
}

func TestProbe_RemoteExistsErrors(t *testing.T) {
	client := &fakeProbeClient{existsErr: errors.New("boom")}
	enabled, reason := Probe(context.Background(), ProbeConfig{
		Enabled:      true,
		Remote:       "myremote",
		PathTemplate: "{year}/{title}.{ext}",
	}, client)
	if enabled || reason != "remote 'myremote' not configured" {
		t.Fatalf("got (%v, %q)", enabled, reason)
	}
}

func TestProbe_AllPass(t *testing.T) {
	client := &fakeProbeClient{existsOK: true}
	enabled, reason := Probe(context.Background(), ProbeConfig{
		Enabled:      true,
		Remote:       "myremote",
		PathTemplate: "{year}/{title}.{ext}",
	}, client)
	if !enabled || reason != "" {
		t.Fatalf("got (%v, %q)", enabled, reason)
	}
}
