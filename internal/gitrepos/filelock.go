package gitrepos

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

var (
	// ErrLockTimeout indicates the lock acquisition timed out
	ErrLockTimeout = errors.New("lock acquisition timed out")

	// ErrLockWouldBlock indicates the lock is held by another process
	ErrLockWouldBlock = errors.New("lock is held by another process")
)

// FileLock provides exclusive file locking using flock(2).
// It is safe for coordination between multiple processes.
// The lock is automatically released when the process exits or crashes.
type FileLock struct {
	path string
	file *os.File
}

// NewFileLock creates a new file lock at the given path.
// The lock file and its parent directories will be created if they don't exist.
func NewFileLock(path string) *FileLock {
	return &FileLock{
		path: path,
	}
}

// TryLock attempts to acquire the exclusive lock without blocking.
// Returns true if the lock was acquired, false if it would block.
// An error is returned only for unexpected failures (not for lock contention).
func (l *FileLock) TryLock() (bool, error) {
	if err := l.ensureFileExists(); err != nil {
		return false, err
	}

	err := syscall.Flock(int(l.file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		if errors.Is(err, syscall.EWOULDBLOCK) {
			// Lock is held by another process - close our file handle
			_ = l.file.Close()
			l.file = nil
			return false, nil
		}
		// Unexpected error
		_ = l.file.Close()
		l.file = nil
		return false, fmt.Errorf("flock failed: %w", err)
	}

	return true, nil
}

// Lock acquires the exclusive lock, blocking until it's available or timeout expires.
// Returns ErrLockTimeout if the timeout expires before the lock is acquired.
func (l *FileLock) Lock(timeout time.Duration) error {
	return l.LockWithContext(context.Background(), timeout)
}

// LockWithContext acquires the exclusive lock, blocking until it's available,
// timeout expires, or the context is canceled.
func (l *FileLock) LockWithContext(ctx context.Context, timeout time.Duration) error {
	if err := l.ensureFileExists(); err != nil {
		return err
	}

	// Create a deadline from the timeout
	deadline := time.Now().Add(timeout)

	// Poll interval - start small and increase
	pollInterval := 10 * time.Millisecond
	maxPollInterval := 500 * time.Millisecond

	for {
		// Check context cancellation
		select {
		case <-ctx.Done():
			_ = l.file.Close()
			l.file = nil
			return ctx.Err()
		default:
		}

		// Check timeout
		if time.Now().After(deadline) {
			_ = l.file.Close()
			l.file = nil
			return ErrLockTimeout
		}

		// Try to acquire lock
		err := syscall.Flock(int(l.file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			// Lock acquired
			return nil
		}

		if !errors.Is(err, syscall.EWOULDBLOCK) {
			// Unexpected error
			_ = l.file.Close()
			l.file = nil
			return fmt.Errorf("flock failed: %w", err)
		}

		// Lock is held, wait and retry
		select {
		case <-ctx.Done():
			_ = l.file.Close()
			l.file = nil
			return ctx.Err()
		case <-time.After(pollInterval):
			// Exponential backoff with cap
			pollInterval = min(pollInterval*2, maxPollInterval)
		}
	}
}

// Unlock releases the lock.
// It is safe to call Unlock on an unlocked FileLock (no-op).
func (l *FileLock) Unlock() error {
	if l.file == nil {
		return nil
	}

	err := syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	closeErr := l.file.Close()
	l.file = nil

	if err != nil {
		return fmt.Errorf("flock unlock failed: %w", err)
	}
	if closeErr != nil {
		return fmt.Errorf("close failed: %w", closeErr)
	}

	return nil
}

// IsLocked returns true if the lock is currently held by this instance.
func (l *FileLock) IsLocked() bool {
	return l.file != nil
}

// Path returns the path to the lock file.
func (l *FileLock) Path() string {
	return l.path
}

// ensureFileExists creates the lock file and its parent directories if needed.
func (l *FileLock) ensureFileExists() error {
	if l.file != nil {
		return nil // Already open
	}

	// Create parent directories
	dir := filepath.Dir(l.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create lock directory: %w", err)
	}

	// Open or create the lock file
	file, err := os.OpenFile(l.path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("failed to open lock file: %w", err)
	}

	l.file = file
	return nil
}
