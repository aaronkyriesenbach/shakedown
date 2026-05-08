package recordings

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Storage defines the interface for audio file storage.
type Storage interface {
	// Write stores data from r at the given path (relative to storage root).
	Write(ctx context.Context, path string, r io.Reader) error
	// Read opens the file at path for reading. Caller must close the returned ReadSeekCloser.
	Read(ctx context.Context, path string) (io.ReadSeekCloser, int64, error)
	// Delete removes the file at path.
	Delete(ctx context.Context, path string) error
	// Exists reports whether the file at path exists.
	Exists(ctx context.Context, path string) (bool, error)
	// MkdirAll creates the directory at path and all parents.
	MkdirAll(ctx context.Context, path string) error
}

// LocalStorage implements Storage using the local filesystem.
type LocalStorage struct {
	root string
}

// NewLocalStorage creates a LocalStorage rooted at root.
// It creates the root directory if it does not exist.
func NewLocalStorage(root string) (*LocalStorage, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("storage: failed to resolve root path: %w", err)
	}
	if err := os.MkdirAll(abs, 0o750); err != nil {
		return nil, fmt.Errorf("storage: failed to create root directory: %w", err)
	}
	return &LocalStorage{root: abs}, nil
}

// SafeJoin joins root with subPath and verifies the result is within root.
// Returns an error if path traversal is detected.
func (s *LocalStorage) SafeJoin(subPath string) (string, error) {
	joined := filepath.Join(s.root, filepath.Clean("/"+subPath))
	// Ensure the resolved path starts with the root (+ separator)
	if !strings.HasPrefix(joined, s.root+string(filepath.Separator)) && joined != s.root {
		return "", fmt.Errorf("storage: path traversal detected: %q", subPath)
	}
	return joined, nil
}

func (s *LocalStorage) Write(ctx context.Context, path string, r io.Reader) error {
	fullPath, err := s.SafeJoin(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o750); err != nil {
		return fmt.Errorf("storage: failed to create directory: %w", err)
	}
	f, err := os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o640)
	if err != nil {
		return fmt.Errorf("storage: failed to open file for writing: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, r); err != nil {
		return fmt.Errorf("storage: failed to write file: %w", err)
	}
	return nil
}

func (s *LocalStorage) Read(_ context.Context, path string) (io.ReadSeekCloser, int64, error) {
	fullPath, err := s.SafeJoin(path)
	if err != nil {
		return nil, 0, err
	}
	f, err := os.Open(fullPath)
	if err != nil {
		return nil, 0, fmt.Errorf("storage: failed to open file: %w", err)
	}
	fi, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, 0, fmt.Errorf("storage: failed to stat file: %w", err)
	}
	return f, fi.Size(), nil
}

func (s *LocalStorage) Delete(_ context.Context, path string) error {
	fullPath, err := s.SafeJoin(path)
	if err != nil {
		return err
	}
	if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("storage: failed to delete file: %w", err)
	}
	return nil
}

func (s *LocalStorage) Exists(_ context.Context, path string) (bool, error) {
	fullPath, err := s.SafeJoin(path)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(fullPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("storage: failed to stat file: %w", err)
	}
	return true, nil
}

func (s *LocalStorage) MkdirAll(_ context.Context, path string) error {
	fullPath, err := s.SafeJoin(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(fullPath, 0o750); err != nil {
		return fmt.Errorf("storage: failed to create directory: %w", err)
	}
	return nil
}
