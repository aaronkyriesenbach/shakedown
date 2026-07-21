package cloudsync

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) (stdout []byte, err error)
}

type defaultCommandRunner struct{}

func (d *defaultCommandRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(bytes.TrimSpace(exitErr.Stderr)) > 0 {
			// Output() captures stderr onto the ExitError but doesn't include it
			// in Error(); surface it so callers see rclone's actual diagnostic
			// (e.g. the underlying API error), not just "exit status 1".
			return out, fmt.Errorf("%w: %s", err, bytes.TrimSpace(exitErr.Stderr))
		}
	}
	return out, err
}

type RemoteClient interface {
	Version(ctx context.Context) (string, error)
	RemoteExists(ctx context.Context) (bool, error)
	WriteRemoteConfig(ctx context.Context, block string) error
	Copy(ctx context.Context, localAbsPath, remotePath string) error
	StatSize(ctx context.Context, remotePath string) (size int64, found bool, err error)
}

type rcloneClient struct {
	runner     CommandRunner
	rcloneBin  string
	configPath string
	remote     string
	tpsLimit   int
}

func NewRcloneClient(runner CommandRunner, rcloneBin, configPath, remote string, tpsLimit int) RemoteClient {
	if runner == nil {
		runner = &defaultCommandRunner{}
	}
	return &rcloneClient{
		runner:     runner,
		rcloneBin:  rcloneBin,
		configPath: configPath,
		remote:     remote,
		tpsLimit:   tpsLimit,
	}
}

// wrapOpError adds rclone-operation context to err without discarding the
// underlying detail (e.g. rclone's stderr, wrapped in by the CommandRunner).
// Callers that need to persist a display-safe summary should go through
// Service.summarizeError rather than truncating/scrubbing here, since this
// is also what ends up in application logs.
func wrapOpError(op string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("rclone %s failed: %w", op, err)
}

func (c *rcloneClient) Version(ctx context.Context) (string, error) {
	out, err := c.runner.Run(ctx, c.rcloneBin, "--config", c.configPath, "version")
	if err != nil {
		return "", wrapOpError("version", err)
	}
	return string(out), nil
}

func (c *rcloneClient) RemoteExists(ctx context.Context) (bool, error) {
	out, err := c.runner.Run(ctx, c.rcloneBin, "--config", c.configPath, "listremotes")
	if err != nil {
		return false, wrapOpError("listremotes", err)
	}
	lines := strings.Split(string(out), "\n")
	target := c.remote + ":"
	return slices.Contains(lines, target), nil
}

func (c *rcloneClient) WriteRemoteConfig(ctx context.Context, block string) error {
	lines := strings.Split(block, "\n")
	foundSection := false
	sectionHeader := "[" + c.remote + "]"

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			if foundSection {
				return errors.New("invalid config block: multiple sections found")
			}
			if line != sectionHeader {
				return errors.New("invalid config block: section name does not match configured remote")
			}
			foundSection = true
		}
	}

	if !foundSection {
		return errors.New("invalid config block: missing section header")
	}

	dir := filepath.Dir(c.configPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	tmpFile := c.configPath + ".tmp"
	if err := os.WriteFile(tmpFile, []byte(block), 0600); err != nil {
		return fmt.Errorf("failed to write temp config: %w", err)
	}

	if err := os.Rename(tmpFile, c.configPath); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("failed to rename config: %w", err)
	}

	return nil
}

func (c *rcloneClient) Copy(ctx context.Context, localAbsPath, remotePath string) error {
	args := []string{"--config", c.configPath, "copyto", localAbsPath, c.remote + ":" + remotePath}
	if c.tpsLimit > 0 {
		args = append(args, "--tpslimit", strconv.Itoa(c.tpsLimit))
	}

	_, err := c.runner.Run(ctx, c.rcloneBin, args...)
	if err != nil {
		return wrapOpError("copyto", err)
	}
	return nil
}

type lsjsonItem struct {
	Size  int64 `json:"Size"`
	IsDir bool  `json:"IsDir"`
}

func (c *rcloneClient) StatSize(ctx context.Context, remotePath string) (int64, bool, error) {
	out, err := c.runner.Run(ctx, c.rcloneBin, "--config", c.configPath, "lsjson", "--stat", c.remote+":"+remotePath)
	if err != nil {
		return 0, false, wrapOpError("lsjson", err)
	}

	out = bytes.TrimSpace(out)
	if len(out) == 0 || string(out) == "[]" {
		return 0, false, nil
	}

	var item lsjsonItem
	if err := json.Unmarshal(out, &item); err != nil {
		return 0, false, wrapOpError("lsjson parse", err)
	}

	if item.IsDir {
		return 0, false, nil
	}

	return item.Size, true, nil
}
