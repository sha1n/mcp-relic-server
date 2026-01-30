package gitrepos

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// unlockLock is a test helper that unlocks and logs any error
func unlockLock(t *testing.T, lock *FileLock) {
	t.Helper()
	if err := lock.Unlock(); err != nil {
		t.Logf("Warning: Unlock failed: %v", err)
	}
}

func TestFileLock_TryLock_Success(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.lock")

	lock := NewFileLock(lockPath)
	defer unlockLock(t, lock)

	acquired, err := lock.TryLock()
	if err != nil {
		t.Fatalf("TryLock failed: %v", err)
	}
	if !acquired {
		t.Error("Expected to acquire lock")
	}
	if !lock.IsLocked() {
		t.Error("Expected IsLocked to return true")
	}
}

func TestFileLock_TryLock_AlreadyHeld(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.lock")

	// Acquire lock with first instance
	lock1 := NewFileLock(lockPath)
	acquired, err := lock1.TryLock()
	if err != nil {
		t.Fatalf("First TryLock failed: %v", err)
	}
	if !acquired {
		t.Fatal("Expected to acquire first lock")
	}
	defer unlockLock(t, lock1)

	// Try to acquire with second instance
	lock2 := NewFileLock(lockPath)
	acquired, err = lock2.TryLock()
	if err != nil {
		t.Fatalf("Second TryLock returned error: %v", err)
	}
	if acquired {
		t.Error("Expected second lock acquisition to fail")
		unlockLock(t, lock2)
	}
	if lock2.IsLocked() {
		t.Error("Expected second lock's IsLocked to return false")
	}
}

func TestFileLock_Lock_Success(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.lock")

	lock := NewFileLock(lockPath)
	defer unlockLock(t, lock)

	err := lock.Lock(1 * time.Second)
	if err != nil {
		t.Fatalf("Lock failed: %v", err)
	}
	if !lock.IsLocked() {
		t.Error("Expected IsLocked to return true")
	}
}

func TestFileLock_Lock_Timeout(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.lock")

	// Acquire lock with first instance
	lock1 := NewFileLock(lockPath)
	acquired, err := lock1.TryLock()
	if err != nil {
		t.Fatalf("First TryLock failed: %v", err)
	}
	if !acquired {
		t.Fatal("Expected to acquire first lock")
	}
	defer unlockLock(t, lock1)

	// Try to acquire with second instance - should timeout
	lock2 := NewFileLock(lockPath)
	start := time.Now()
	err = lock2.Lock(100 * time.Millisecond)
	elapsed := time.Since(start)

	if err != ErrLockTimeout {
		t.Errorf("Expected ErrLockTimeout, got: %v", err)
	}
	if elapsed < 100*time.Millisecond {
		t.Errorf("Expected at least 100ms to elapse, got %v", elapsed)
	}
}

func TestFileLock_Lock_AcquiresAfterRelease(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.lock")

	lock1 := NewFileLock(lockPath)
	lock2 := NewFileLock(lockPath)

	// Acquire first lock
	acquired, err := lock1.TryLock()
	if err != nil || !acquired {
		t.Fatalf("Failed to acquire first lock: %v", err)
	}

	// Start second lock attempt in goroutine
	var wg sync.WaitGroup
	var lock2Err error
	wg.Add(1)
	go func() {
		defer wg.Done()
		lock2Err = lock2.Lock(2 * time.Second)
	}()

	// Release first lock after short delay
	time.Sleep(100 * time.Millisecond)
	if err := lock1.Unlock(); err != nil {
		t.Fatalf("Failed to unlock first lock: %v", err)
	}

	// Wait for second lock attempt
	wg.Wait()

	if lock2Err != nil {
		t.Errorf("Expected second lock to succeed after release, got: %v", lock2Err)
	}
	if !lock2.IsLocked() {
		t.Error("Expected second lock to be held")
	}
	unlockLock(t, lock2)
}

func TestFileLock_LockWithContext_Cancellation(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.lock")

	// Acquire lock with first instance
	lock1 := NewFileLock(lockPath)
	acquired, err := lock1.TryLock()
	if err != nil || !acquired {
		t.Fatalf("Failed to acquire first lock: %v", err)
	}
	defer unlockLock(t, lock1)

	// Try to acquire with cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	lock2 := NewFileLock(lockPath)

	var wg sync.WaitGroup
	var lock2Err error
	wg.Add(1)
	go func() {
		defer wg.Done()
		lock2Err = lock2.LockWithContext(ctx, 10*time.Second)
	}()

	// Cancel after short delay
	time.Sleep(100 * time.Millisecond)
	cancel()
	wg.Wait()

	if lock2Err != context.Canceled {
		t.Errorf("Expected context.Canceled, got: %v", lock2Err)
	}
}

func TestFileLock_Unlock_ReleasesProperly(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.lock")

	lock1 := NewFileLock(lockPath)
	acquired, err := lock1.TryLock()
	if err != nil || !acquired {
		t.Fatalf("Failed to acquire lock: %v", err)
	}

	err = lock1.Unlock()
	if err != nil {
		t.Fatalf("Unlock failed: %v", err)
	}
	if lock1.IsLocked() {
		t.Error("Expected IsLocked to return false after unlock")
	}

	// Should be able to acquire again
	lock2 := NewFileLock(lockPath)
	acquired, err = lock2.TryLock()
	if err != nil {
		t.Fatalf("Second TryLock failed: %v", err)
	}
	if !acquired {
		t.Error("Expected to acquire lock after unlock")
	}
	unlockLock(t, lock2)
}

func TestFileLock_Unlock_NoOp(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.lock")

	lock := NewFileLock(lockPath)

	// Unlock without ever locking - should be no-op
	err := lock.Unlock()
	if err != nil {
		t.Errorf("Expected no error for no-op unlock, got: %v", err)
	}
}

func TestFileLock_Unlock_MultipleTimes(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.lock")

	lock := NewFileLock(lockPath)
	acquired, err := lock.TryLock()
	if err != nil || !acquired {
		t.Fatalf("Failed to acquire lock: %v", err)
	}

	// First unlock
	if err := lock.Unlock(); err != nil {
		t.Errorf("First unlock failed: %v", err)
	}

	// Second unlock - should be no-op
	if err := lock.Unlock(); err != nil {
		t.Errorf("Second unlock should be no-op, got: %v", err)
	}
}

func TestFileLock_CreatesDirectories(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "nested", "dirs", "test.lock")

	lock := NewFileLock(lockPath)
	defer unlockLock(t, lock)

	acquired, err := lock.TryLock()
	if err != nil {
		t.Fatalf("TryLock failed: %v", err)
	}
	if !acquired {
		t.Error("Expected to acquire lock")
	}

	// Verify directories were created
	info, err := os.Stat(filepath.Dir(lockPath))
	if err != nil {
		t.Fatalf("Parent directory should exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("Expected parent path to be a directory")
	}
}

func TestFileLock_Path(t *testing.T) {
	lockPath := "/some/path/to/lock.file"
	lock := NewFileLock(lockPath)

	if lock.Path() != lockPath {
		t.Errorf("Path() = %q, want %q", lock.Path(), lockPath)
	}
}

func TestFileLock_ConcurrentGoroutines(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "concurrent.lock")

	const numGoroutines = 10
	const opsPerGoroutine = 5

	var wg sync.WaitGroup
	successCount := make(chan int, numGoroutines*opsPerGoroutine)

	for range numGoroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range opsPerGoroutine {
				lock := NewFileLock(lockPath)
				err := lock.Lock(5 * time.Second)
				if err != nil {
					t.Errorf("Lock failed: %v", err)
					return
				}

				// Hold lock briefly
				time.Sleep(time.Millisecond)
				successCount <- 1

				if err := lock.Unlock(); err != nil {
					t.Errorf("Unlock failed: %v", err)
				}
			}
		}()
	}

	wg.Wait()
	close(successCount)

	total := 0
	for range successCount {
		total++
	}

	expected := numGoroutines * opsPerGoroutine
	if total != expected {
		t.Errorf("Expected %d successful operations, got %d", expected, total)
	}
}

func TestFileLock_CrossProcess(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping cross-process test in short mode")
	}

	// Check if flock command is available (not on macOS by default)
	if _, err := exec.LookPath("flock"); err != nil {
		t.Skip("Skipping cross-process test: flock command not available")
	}

	dir := t.TempDir()
	lockPath := filepath.Join(dir, "crossprocess.lock")

	// Acquire lock in parent process
	lock := NewFileLock(lockPath)
	acquired, err := lock.TryLock()
	if err != nil || !acquired {
		t.Fatalf("Failed to acquire lock in parent: %v", err)
	}
	defer unlockLock(t, lock)

	// Try to acquire in child process - should fail
	cmd := exec.Command("sh", "-c", `
		flock -n "$1" -c "echo acquired" 2>/dev/null || echo "blocked"
	`, "_", lockPath)
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Child process failed: %v", err)
	}

	result := string(output)
	if result != "blocked\n" {
		t.Errorf("Expected child to be blocked, got: %q", result)
	}
}

func TestFileLock_ReleaseOnUnlock_AllowsNewProcess(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping cross-process test in short mode")
	}

	// Check if flock command is available (not on macOS by default)
	if _, err := exec.LookPath("flock"); err != nil {
		t.Skip("Skipping cross-process test: flock command not available")
	}

	dir := t.TempDir()
	lockPath := filepath.Join(dir, "release.lock")

	// Acquire and release lock
	lock := NewFileLock(lockPath)
	acquired, err := lock.TryLock()
	if err != nil || !acquired {
		t.Fatalf("Failed to acquire lock: %v", err)
	}
	if err := lock.Unlock(); err != nil {
		t.Fatalf("Failed to release lock: %v", err)
	}

	// Try to acquire in child process - should succeed
	cmd := exec.Command("sh", "-c", `
		flock -n "$1" -c "echo acquired" 2>/dev/null || echo "blocked"
	`, "_", lockPath)
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Child process failed: %v", err)
	}

	result := string(output)
	if result != "acquired\n" {
		t.Errorf("Expected child to acquire lock, got: %q", result)
	}
}
