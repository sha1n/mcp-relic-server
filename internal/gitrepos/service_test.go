package gitrepos

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sha1n/mcp-relic-server/internal/config"
)

// ============================
// NewService tests (real constructor, no deps needed)
// ============================

func TestNewService(t *testing.T) {
	dir := t.TempDir()
	settings := &config.GitReposSettings{
		URLs:         []string{"git@github.com:test/repo.git"},
		BaseDir:      dir,
		SyncInterval: 15 * time.Minute,
		SyncTimeout:  60 * time.Second,
		MaxFileSize:  256 * 1024,
		MaxResults:   20,
	}

	svc, err := NewService(settings)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	defer func() {
		if err := svc.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	if svc == nil {
		t.Fatal("Expected non-nil service")
	}

	// Verify directories were created
	reposDir := filepath.Join(dir, "repos")
	if _, err := os.Stat(reposDir); err != nil {
		t.Errorf("Repos directory should exist: %v", err)
	}

	indexesDir := filepath.Join(dir, "indexes")
	if _, err := os.Stat(indexesDir); err != nil {
		t.Errorf("Indexes directory should exist: %v", err)
	}
}

func TestNewService_NilSettings(t *testing.T) {
	_, err := NewService(nil)
	if err == nil {
		t.Error("Expected error for nil settings")
	}
}

func TestNewService_CreatesDirStructure(t *testing.T) {
	dir := t.TempDir()
	baseDir := filepath.Join(dir, "nested", "path", "base")

	settings := &config.GitReposSettings{
		BaseDir:     baseDir,
		MaxFileSize: 256 * 1024,
	}

	svc, err := NewService(settings)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	defer func() {
		if err := svc.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	if _, err := os.Stat(baseDir); err != nil {
		t.Errorf("Base directory should exist: %v", err)
	}
}

// ============================
// Service method tests (using real NewService)
// ============================

func TestService_IsReady_InitiallyFalse(t *testing.T) {
	dir := t.TempDir()
	settings := &config.GitReposSettings{
		BaseDir:     dir,
		MaxFileSize: 256 * 1024,
	}

	svc, err := NewService(settings)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	defer func() {
		if err := svc.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	if svc.IsReady() {
		t.Error("Service should not be ready before initialization")
	}
}

func TestService_GetIndexAlias_NotReady(t *testing.T) {
	dir := t.TempDir()
	settings := &config.GitReposSettings{
		BaseDir:     dir,
		MaxFileSize: 256 * 1024,
	}

	svc, err := NewService(settings)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	defer func() {
		if err := svc.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	_, err = svc.GetIndexAlias()
	if err == nil {
		t.Error("Expected error when getting alias before ready")
	}
}

func TestService_GetRepoDir(t *testing.T) {
	dir := t.TempDir()
	settings := &config.GitReposSettings{
		BaseDir:     dir,
		MaxFileSize: 256 * 1024,
	}

	svc, err := NewService(settings)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	defer func() {
		if err := svc.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	repoDir := svc.GetRepoDir("github.com_test_repo")
	expected := filepath.Join(dir, "repos", "github.com_test_repo")
	if repoDir != expected {
		t.Errorf("GetRepoDir = %q, want %q", repoDir, expected)
	}
}

func TestService_MaxResults(t *testing.T) {
	dir := t.TempDir()
	settings := &config.GitReposSettings{
		BaseDir:     dir,
		MaxFileSize: 256 * 1024,
		MaxResults:  42,
	}

	svc, err := NewService(settings)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	defer func() {
		if err := svc.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	if got := svc.MaxResults(); got != 42 {
		t.Errorf("MaxResults() = %d, want 42", got)
	}
}

func TestService_MaxFileSize(t *testing.T) {
	dir := t.TempDir()
	settings := &config.GitReposSettings{
		BaseDir:     dir,
		MaxFileSize: 512 * 1024,
		MaxResults:  20,
	}

	svc, err := NewService(settings)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	defer func() {
		if err := svc.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	if got := svc.MaxFileSize(); got != 512*1024 {
		t.Errorf("MaxFileSize() = %d, want %d", got, 512*1024)
	}
}

func TestService_GetSettings(t *testing.T) {
	dir := t.TempDir()
	settings := &config.GitReposSettings{
		BaseDir:     dir,
		MaxFileSize: 256 * 1024,
		MaxResults:  42,
	}

	svc, err := NewService(settings)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	defer func() {
		if err := svc.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	got := svc.GetSettings()
	if got.MaxResults != 42 {
		t.Errorf("GetSettings().MaxResults = %d, want 42", got.MaxResults)
	}
}

func TestService_Close(t *testing.T) {
	dir := t.TempDir()
	settings := &config.GitReposSettings{
		BaseDir:     dir,
		MaxFileSize: 256 * 1024,
	}

	svc, err := NewService(settings)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	if err := svc.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Close again should be safe
	if err := svc.Close(); err != nil {
		t.Errorf("Second Close returned error: %v", err)
	}
}

func TestService_SetGitOperations(t *testing.T) {
	svc := &Service{}
	client := NewGitClient()
	svc.SetGitOperations(client)
	if svc.git != client {
		t.Error("SetGitOperations did not set the client")
	}
}

func TestRegisterTools(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "1.0"}, nil)

	dir := t.TempDir()
	settings := &config.GitReposSettings{BaseDir: dir}
	svc, _ := NewService(settings)

	RegisterSearchTool(server, svc)
	RegisterReadTool(server, svc)
}

// ============================
// Initialize tests with mocked dependencies
// ============================

func TestService_Initialize_LockError(t *testing.T) {
	svc := NewServiceWithDeps(
		&config.GitReposSettings{BaseDir: t.TempDir()},
		ServiceDeps{
			Lock: &mockSyncLock{tryLockErr: fmt.Errorf("lock broken")},
		},
	)

	err := svc.Initialize(context.Background())
	if err == nil {
		t.Fatal("Expected error when TryLock fails")
	}
	if !strings.Contains(err.Error(), "lock broken") {
		t.Errorf("Expected 'lock broken' in error, got: %v", err)
	}
}

func TestService_Initialize_LeaderNoURLs(t *testing.T) {
	svc := NewServiceWithDeps(
		&config.GitReposSettings{
			BaseDir: t.TempDir(),
			URLs:    []string{},
		},
		ServiceDeps{
			Git:      &mockGitOps{},
			Indexer:  &mockIndexOps{},
			Manifest: newMockManifestOps(),
			Lock:     &mockSyncLock{tryLockResult: true},
		},
	)

	err := svc.Initialize(context.Background())
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	if svc.IsReady() {
		t.Error("Service should not be ready with no URLs")
	}
}

func TestService_Initialize_LeaderSyncSuccess(t *testing.T) {
	dir := t.TempDir()
	repoID := "github.com_test_repo"

	svc := NewServiceWithDeps(
		&config.GitReposSettings{
			BaseDir:     dir,
			URLs:        []string{"git@github.com:test/repo.git"},
			SyncTimeout: 5 * time.Second,
		},
		ServiceDeps{
			Git:      &mockGitOps{headCommit: "abc123"},
			Indexer:  &mockIndexOps{fullIndexCount: 5, existsMap: map[string]bool{repoID: true}},
			Manifest: newMockManifestOps(),
			Lock:     &mockSyncLock{tryLockResult: true},
		},
	)

	err := svc.Initialize(context.Background())
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	// Note: ready depends on indexer.CreateAlias which returns nil alias here.
	// In real scenario the alias would be non-nil. For this test the mock returns nil alias,
	// so CreateAlias will return an error. We're testing the leader path executes.
}

func TestService_Initialize_LeaderSyncFails_ReturnsError(t *testing.T) {
	svc := NewServiceWithDeps(
		&config.GitReposSettings{
			BaseDir:     t.TempDir(),
			URLs:        []string{"git@github.com:test/repo.git"},
			SyncTimeout: 5 * time.Second,
		},
		ServiceDeps{
			Git:      &mockGitOps{cloneErr: fmt.Errorf("clone fail")},
			Indexer:  &mockIndexOps{},
			Manifest: newMockManifestOps(),
			Lock:     &mockSyncLock{tryLockResult: true},
		},
	)

	// Should return error when sync fails and no indexes are available
	err := svc.Initialize(context.Background())
	if err == nil {
		t.Fatal("Expected error when sync fails and no indexes are available")
	}
	if !strings.Contains(err.Error(), "git repos initialization failed") {
		t.Errorf("Expected error containing 'git repos initialization failed', got: %v", err)
	}
}

func TestService_Initialize_LeaderManifestSaveError(t *testing.T) {
	manifest := newMockManifestOps()
	manifest.saveErr = fmt.Errorf("disk full")

	svc := NewServiceWithDeps(
		&config.GitReposSettings{
			BaseDir:     t.TempDir(),
			URLs:        []string{},
			SyncTimeout: 5 * time.Second,
		},
		ServiceDeps{
			Git:      &mockGitOps{},
			Indexer:  &mockIndexOps{},
			Manifest: manifest,
			Lock:     &mockSyncLock{tryLockResult: true},
		},
	)

	// Manifest save error is logged, not returned
	err := svc.Initialize(context.Background())
	if err != nil {
		t.Fatalf("Initialize should not fail for manifest save error: %v", err)
	}
}

func TestService_Initialize_LeaderUnlockError(t *testing.T) {
	svc := NewServiceWithDeps(
		&config.GitReposSettings{
			BaseDir:     t.TempDir(),
			URLs:        []string{},
			SyncTimeout: 5 * time.Second,
		},
		ServiceDeps{
			Git:      &mockGitOps{},
			Indexer:  &mockIndexOps{},
			Manifest: newMockManifestOps(),
			Lock:     &mockSyncLock{tryLockResult: true, unlockErr: fmt.Errorf("unlock fail")},
		},
	)

	// Unlock error is logged, not returned
	err := svc.Initialize(context.Background())
	if err != nil {
		t.Fatalf("Initialize should not fail for unlock error: %v", err)
	}
}

func TestService_Initialize_FollowerTimeout(t *testing.T) {
	svc := NewServiceWithDeps(
		&config.GitReposSettings{
			BaseDir:     t.TempDir(),
			URLs:        []string{},
			SyncTimeout: 1 * time.Second,
		},
		ServiceDeps{
			Git:      &mockGitOps{},
			Indexer:  &mockIndexOps{},
			Manifest: newMockManifestOps(),
			Lock:     &mockSyncLock{tryLockResult: false, lockErr: ErrLockTimeout},
		},
	)

	// Follower timeout is logged, not returned (tries to open indexes)
	err := svc.Initialize(context.Background())
	if err != nil {
		t.Fatalf("Initialize should not fail for follower timeout: %v", err)
	}
}

func TestService_Initialize_FollowerSuccess(t *testing.T) {
	svc := NewServiceWithDeps(
		&config.GitReposSettings{
			BaseDir:     t.TempDir(),
			URLs:        []string{},
			SyncTimeout: 1 * time.Second,
		},
		ServiceDeps{
			Git:      &mockGitOps{},
			Indexer:  &mockIndexOps{},
			Manifest: newMockManifestOps(),
			Lock:     &mockSyncLock{tryLockResult: false, lockErr: nil},
		},
	)

	err := svc.Initialize(context.Background())
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
}

func TestService_Initialize_FollowerUnlockError(t *testing.T) {
	svc := NewServiceWithDeps(
		&config.GitReposSettings{
			BaseDir:     t.TempDir(),
			URLs:        []string{},
			SyncTimeout: 1 * time.Second,
		},
		ServiceDeps{
			Git:      &mockGitOps{},
			Indexer:  &mockIndexOps{},
			Manifest: newMockManifestOps(),
			Lock:     &mockSyncLock{tryLockResult: false, lockErr: nil, unlockErr: fmt.Errorf("unlock fail")},
		},
	)

	// Unlock error is logged, not returned
	err := svc.Initialize(context.Background())
	if err != nil {
		t.Fatalf("Initialize should not fail for unlock error: %v", err)
	}
}

// ============================
// OpenIndexes tests via Initialize
// ============================

func TestService_OpenIndexes_NoIndexes(t *testing.T) {
	svc := NewServiceWithDeps(
		&config.GitReposSettings{
			BaseDir: t.TempDir(),
			URLs:    []string{"git@github.com:test/repo.git"},
		},
		ServiceDeps{
			Git:      &mockGitOps{headCommit: "abc123"},
			Indexer:  &mockIndexOps{existsMap: map[string]bool{}},
			Manifest: newMockManifestOps(),
			Lock:     &mockSyncLock{tryLockResult: true},
		},
	)

	// Should return error when URLs are configured but no indexes are available
	err := svc.Initialize(context.Background())
	if err == nil {
		t.Fatal("Expected error when no indexes are available")
	}
	if !strings.Contains(err.Error(), "no indexes available") {
		t.Errorf("Expected error containing 'no indexes available', got: %v", err)
	}
	if svc.IsReady() {
		t.Error("Service should not be ready when no indexes exist")
	}
}

func TestService_OpenIndexes_AliasError(t *testing.T) {
	repoID := "github.com_test_repo"
	svc := NewServiceWithDeps(
		&config.GitReposSettings{
			BaseDir: t.TempDir(),
			URLs:    []string{"git@github.com:test/repo.git"},
		},
		ServiceDeps{
			Git: &mockGitOps{headCommit: "abc123"},
			Indexer: &mockIndexOps{
				existsMap: map[string]bool{repoID: true},
				aliasErr:  fmt.Errorf("alias creation failed"),
			},
			Manifest: newMockManifestOps(),
			Lock:     &mockSyncLock{tryLockResult: true},
		},
	)

	err := svc.Initialize(context.Background())
	if err == nil {
		t.Fatal("Expected error when alias creation fails")
	}
	if !strings.Contains(err.Error(), "failed to create index alias") {
		t.Errorf("Expected 'failed to create index alias' in error, got: %v", err)
	}
}

// ============================
// SyncAll tests with mocked deps
// ============================

func TestService_SyncAll_NoURLs(t *testing.T) {
	svc := NewServiceWithDeps(
		&config.GitReposSettings{
			BaseDir: t.TempDir(),
			URLs:    []string{},
		},
		ServiceDeps{
			Git:      &mockGitOps{},
			Indexer:  &mockIndexOps{},
			Manifest: newMockManifestOps(),
			Lock:     &mockSyncLock{},
		},
	)

	if err := svc.SyncAll(context.Background()); err != nil {
		t.Errorf("SyncAll with no URLs should succeed: %v", err)
	}
}

func TestService_SyncRepo_CloneError(t *testing.T) {
	svc := NewServiceWithDeps(
		&config.GitReposSettings{
			BaseDir: t.TempDir(),
			URLs:    []string{"git@github.com:test/repo.git"},
		},
		ServiceDeps{
			Git:      &mockGitOps{cloneErr: fmt.Errorf("network error")},
			Indexer:  &mockIndexOps{},
			Manifest: newMockManifestOps(),
			Lock:     &mockSyncLock{},
		},
	)

	err := svc.SyncAll(context.Background())
	if err == nil {
		t.Fatal("Expected error when clone fails")
	}
}

func TestService_SyncRepo_FetchError(t *testing.T) {
	manifest := newMockManifestOps()
	repoID := "github.com_test_repo"
	manifest.repos[repoID] = RepoState{
		URL:      "git@github.com:test/repo.git",
		ClonedAt: time.Now().Add(-1 * time.Hour),
	}

	svc := NewServiceWithDeps(
		&config.GitReposSettings{
			BaseDir: t.TempDir(),
			URLs:    []string{"git@github.com:test/repo.git"},
		},
		ServiceDeps{
			Git:      &mockGitOps{fetchErr: fmt.Errorf("fetch failed")},
			Indexer:  &mockIndexOps{},
			Manifest: manifest,
			Lock:     &mockSyncLock{},
		},
	)

	err := svc.SyncAll(context.Background())
	if err == nil {
		t.Fatal("Expected error when fetch fails")
	}
}

func TestService_SyncRepo_HeadCommitError(t *testing.T) {
	svc := NewServiceWithDeps(
		&config.GitReposSettings{
			BaseDir: t.TempDir(),
			URLs:    []string{"git@github.com:test/repo.git"},
		},
		ServiceDeps{
			Git:      &mockGitOps{headCommitErr: fmt.Errorf("rev-parse failed")},
			Indexer:  &mockIndexOps{},
			Manifest: newMockManifestOps(),
			Lock:     &mockSyncLock{},
		},
	)

	err := svc.SyncAll(context.Background())
	if err == nil {
		t.Fatal("Expected error when GetHeadCommit fails")
	}
}

func TestService_SyncRepo_ResetError(t *testing.T) {
	manifest := newMockManifestOps()
	repoID := "github.com_test_repo"
	manifest.repos[repoID] = RepoState{
		URL:         "git@github.com:test/repo.git",
		ClonedAt:    time.Now().Add(-1 * time.Hour),
		LastCommit:  "commit1",
		LastIndexed: "commit1",
	}

	svc := NewServiceWithDeps(
		&config.GitReposSettings{
			BaseDir: t.TempDir(),
			URLs:    []string{"git@github.com:test/repo.git"},
		},
		ServiceDeps{
			Git:      &mockGitOps{headCommit: "commit2", resetErr: fmt.Errorf("reset failed")},
			Indexer:  &mockIndexOps{},
			Manifest: manifest,
			Lock:     &mockSyncLock{},
		},
	)

	err := svc.SyncAll(context.Background())
	if err == nil {
		t.Fatal("Expected error when reset fails")
	}
}

func TestService_SyncRepo_FullIndexError(t *testing.T) {
	svc := NewServiceWithDeps(
		&config.GitReposSettings{
			BaseDir: t.TempDir(),
			URLs:    []string{"git@github.com:test/repo.git"},
		},
		ServiceDeps{
			Git:      &mockGitOps{headCommit: "abc123"},
			Indexer:  &mockIndexOps{fullIndexErr: fmt.Errorf("index failed")},
			Manifest: newMockManifestOps(),
			Lock:     &mockSyncLock{},
		},
	)

	err := svc.SyncAll(context.Background())
	if err == nil {
		t.Fatal("Expected error when full index fails")
	}
}

func TestService_SyncRepo_IncrementalFails_FallsBackToFull(t *testing.T) {
	manifest := newMockManifestOps()
	repoID := "github.com_test_repo"
	manifest.repos[repoID] = RepoState{
		URL:         "git@github.com:test/repo.git",
		ClonedAt:    time.Now().Add(-1 * time.Hour),
		LastCommit:  "commit1",
		LastIndexed: "commit1",
	}

	svc := NewServiceWithDeps(
		&config.GitReposSettings{
			BaseDir: t.TempDir(),
			URLs:    []string{"git@github.com:test/repo.git"},
		},
		ServiceDeps{
			Git: &mockGitOps{
				headCommit:   "commit2",
				changedFiles: []string{"file1.go"},
			},
			Indexer: &mockIndexOps{
				incrIndexErr:   fmt.Errorf("incremental failed"),
				fullIndexCount: 5,
			},
			Manifest: manifest,
			Lock:     &mockSyncLock{},
		},
	)

	err := svc.SyncAll(context.Background())
	if err != nil {
		t.Fatalf("SyncAll should succeed with fallback to full index: %v", err)
	}

	state := manifest.repos[repoID]
	if state.LastCommit != "commit2" {
		t.Errorf("Expected LastCommit = 'commit2', got %q", state.LastCommit)
	}
	if state.FileCount != 5 {
		t.Errorf("Expected FileCount = 5, got %d", state.FileCount)
	}
}

// ============================
// Tests using real NewService + MockExecutor (for testing real flows)
// ============================

func TestService_SyncAll_WithMockGit(t *testing.T) {
	dir := t.TempDir()
	settings := &config.GitReposSettings{
		URLs:        []string{"git@github.com:test/repo.git"},
		BaseDir:     dir,
		SyncTimeout: 5 * time.Second,
		MaxFileSize: 256 * 1024,
	}

	svc, err := NewService(settings)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	defer func() {
		if err := svc.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	mock := NewMockExecutor()
	mock.AddResponse("git clone", []byte{}, nil)
	mock.AddResponse("git rev-parse", []byte("abc123def456\n"), nil)
	svc.git = NewGitClientWithExecutor(mock)

	repoDir := filepath.Join(dir, "repos", "github.com_test_repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("Failed to create repo dir: %v", err)
	}

	testFile := filepath.Join(repoDir, "main.go")
	if err := os.WriteFile(testFile, []byte("package main\n\nfunc main() {}"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	ctx := context.Background()
	err = svc.SyncAll(ctx)
	if err != nil {
		t.Logf("SyncAll returned error (expected with mock): %v", err)
	}
}

func TestService_Initialize_LeaderSync(t *testing.T) {
	dir := t.TempDir()
	settings := &config.GitReposSettings{
		URLs:        []string{"git@github.com:test/repo.git"},
		BaseDir:     dir,
		SyncTimeout: 1 * time.Second,
		MaxFileSize: 256 * 1024,
	}

	svc, err := NewService(settings)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	defer func() {
		if err := svc.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	mock := NewMockExecutor()
	mock.AddResponse("git clone", []byte{}, nil)
	mock.AddResponse("git rev-parse", []byte("abc123\n"), nil)
	svc.git = NewGitClientWithExecutor(mock)

	repoDir := filepath.Join(dir, "repos", "github.com_test_repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("Failed to create repo dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "test.go"), []byte("package test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	ctx := context.Background()
	err = svc.Initialize(ctx)
	if err != nil {
		t.Logf("Initialize returned error (expected with mock): %v", err)
	}
}

func TestService_RemovesStaleRepos(t *testing.T) {
	dir := t.TempDir()
	settings := &config.GitReposSettings{
		URLs:        []string{"git@github.com:test/repo1.git"},
		BaseDir:     dir,
		MaxFileSize: 256 * 1024,
	}

	svc, err := NewService(settings)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	defer func() {
		if err := svc.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	// Access the concrete manifest through GetSettings and the manifest field
	manifest := svc.manifest.(*Manifest)
	manifest.SetRepoState("github.com_old_repo", RepoState{
		URL:      "git@github.com:old/repo.git",
		ClonedAt: time.Now(),
	})

	staleRepoDir := filepath.Join(dir, "repos", "github.com_old_repo")
	if err := os.MkdirAll(staleRepoDir, 0755); err != nil {
		t.Fatalf("Failed to create stale repo dir: %v", err)
	}

	mock := NewMockExecutor()
	mock.AddResponse("git clone", []byte{}, nil)
	mock.AddResponse("git rev-parse", []byte("abc123\n"), nil)
	svc.git = NewGitClientWithExecutor(mock)

	repoDir := filepath.Join(dir, "repos", "github.com_test_repo1")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("Failed to create repo dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "test.go"), []byte("package test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	ctx := context.Background()
	_ = svc.SyncAll(ctx)

	if manifest.HasRepo("github.com_old_repo") {
		t.Error("Stale repo should have been removed from manifest")
	}

	if _, err := os.Stat(staleRepoDir); !os.IsNotExist(err) {
		t.Error("Stale repo directory should have been removed")
	}
}

func TestService_IndexesReadyAfterSync(t *testing.T) {
	dir := t.TempDir()
	settings := &config.GitReposSettings{
		URLs:        []string{"git@github.com:test/repo.git"},
		BaseDir:     dir,
		SyncTimeout: 5 * time.Second,
		MaxFileSize: 256 * 1024,
	}

	svc, err := NewService(settings)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	defer func() {
		if err := svc.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	mock := NewMockExecutor()
	mock.AddResponse("git clone", []byte{}, nil)
	mock.AddResponse("git rev-parse", []byte("abc123\n"), nil)
	svc.git = NewGitClientWithExecutor(mock)

	repoDir := filepath.Join(dir, "repos", "github.com_test_repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("Failed to create repo dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "main.go"), []byte("package main\nfunc main() {}"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	ctx := context.Background()
	_ = svc.Initialize(ctx)

	if !svc.IsReady() {
		t.Error("Service should be ready after successful initialization")
	}

	alias, err := svc.GetIndexAlias()
	if err != nil {
		t.Errorf("GetIndexAlias failed: %v", err)
	}
	if alias == nil {
		t.Error("Expected non-nil alias")
	}
}

func TestService_ContextCancellation(t *testing.T) {
	dir := t.TempDir()
	settings := &config.GitReposSettings{
		URLs:        []string{"git@github.com:test/repo.git"},
		BaseDir:     dir,
		SyncTimeout: 5 * time.Second,
		MaxFileSize: 256 * 1024,
	}

	svc, err := NewService(settings)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	defer func() {
		if err := svc.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	mock := NewMockExecutor()
	mock.AddResponse("git clone", []byte{}, context.Canceled)
	svc.git = NewGitClientWithExecutor(mock)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = svc.SyncAll(ctx)
	t.Logf("SyncAll with cancelled context: %v", err)
}

func TestService_ParallelSync(t *testing.T) {
	dir := t.TempDir()
	settings := &config.GitReposSettings{
		URLs: []string{
			"git@github.com:test/repo1.git",
			"git@github.com:test/repo2.git",
			"git@github.com:test/repo3.git",
			"git@github.com:test/repo4.git",
			"git@github.com:test/repo5.git",
		},
		BaseDir:     dir,
		SyncTimeout: 5 * time.Second,
		MaxFileSize: 256 * 1024,
	}

	svc, err := NewService(settings)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	defer func() {
		if err := svc.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	mock := NewMockExecutor()
	for i := 0; i < 5; i++ {
		mock.AddResponse("git clone", []byte{}, nil)
		mock.AddResponse("git rev-parse", []byte("abc123\n"), nil)
	}
	svc.git = NewGitClientWithExecutor(mock)

	for i := 1; i <= 5; i++ {
		repoID := URLToRepoID(settings.URLs[i-1])
		repoDir := filepath.Join(dir, "repos", repoID)
		if err := os.MkdirAll(repoDir, 0755); err != nil {
			t.Fatalf("Failed to create repo dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoDir, "main.go"), []byte("package main"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	ctx := context.Background()
	start := time.Now()
	err = svc.SyncAll(ctx)
	elapsed := time.Since(start)

	t.Logf("Parallel sync completed in %v, error: %v", elapsed, err)
}

func TestService_IncrementalIndex(t *testing.T) {
	dir := t.TempDir()
	settings := &config.GitReposSettings{
		URLs:        []string{"git@github.com:test/repo.git"},
		BaseDir:     dir,
		SyncTimeout: 5 * time.Second,
		MaxFileSize: 256 * 1024,
	}

	svc, err := NewService(settings)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	defer func() {
		if err := svc.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	repoID := "github.com_test_repo"
	repoDir := filepath.Join(dir, "repos", repoID)
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("Failed to create repo dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "main.go"), []byte("package main"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	mock := NewMockExecutor()
	mock.AddResponse("git clone", []byte{}, nil)
	mock.AddResponse("git rev-parse", []byte("commit1\n"), nil)
	svc.git = NewGitClientWithExecutor(mock)

	ctx := context.Background()
	_ = svc.SyncAll(ctx)

	mock2 := NewMockExecutor()
	mock2.AddResponse("git fetch", []byte{}, nil)
	mock2.AddResponse("git rev-parse", []byte("commit2\n"), nil)
	mock2.AddResponse("git reset", []byte{}, nil)
	mock2.AddResponse("git diff", []byte("main.go\n"), nil)
	svc.git = NewGitClientWithExecutor(mock2)

	manifest := svc.manifest.(*Manifest)
	manifest.SetRepoState(repoID, RepoState{
		URL:         "git@github.com:test/repo.git",
		ClonedAt:    time.Now().Add(-1 * time.Hour),
		LastCommit:  "commit1",
		LastIndexed: "commit1",
	})

	_ = svc.SyncAll(ctx)

	state := manifest.GetRepoState(repoID)
	if state.LastCommit != "commit2" {
		t.Errorf("Expected LastCommit to be 'commit2', got %q", state.LastCommit)
	}
}

func TestService_IncrementalIndex_ThresholdExceeded(t *testing.T) {
	dir := t.TempDir()
	settings := &config.GitReposSettings{
		URLs:        []string{"git@github.com:test/repo.git"},
		BaseDir:     dir,
		SyncTimeout: 5 * time.Second,
		MaxFileSize: 256 * 1024,
	}

	svc, err := NewService(settings)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	defer func() {
		if err := svc.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	repoID := "github.com_test_repo"
	repoDir := filepath.Join(dir, "repos", repoID)
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("Failed to create repo dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "main.go"), []byte("package main"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	mock := NewMockExecutor()
	mock.AddResponse("git clone", []byte{}, nil)
	mock.AddResponse("git rev-parse", []byte("commit1\n"), nil)
	svc.git = NewGitClientWithExecutor(mock)

	ctx := context.Background()
	_ = svc.SyncAll(ctx)

	var diffLines strings.Builder
	for i := 0; i < 101; i++ {
		fmt.Fprintf(&diffLines, "file%d.go\n", i)
	}

	mock2 := NewMockExecutor()
	mock2.AddResponse("git fetch", []byte{}, nil)
	mock2.AddResponse("git rev-parse", []byte("commit2\n"), nil)
	mock2.AddResponse("git reset", []byte{}, nil)
	mock2.AddResponse("git diff", []byte(diffLines.String()), nil)
	svc.git = NewGitClientWithExecutor(mock2)

	manifest := svc.manifest.(*Manifest)
	manifest.SetRepoState(repoID, RepoState{
		URL:         "git@github.com:test/repo.git",
		ClonedAt:    time.Now().Add(-1 * time.Hour),
		LastCommit:  "commit1",
		LastIndexed: "commit1",
		FileCount:   1,
	})

	_ = svc.SyncAll(ctx)

	state := manifest.GetRepoState(repoID)
	if state.LastCommit != "commit2" {
		t.Errorf("Expected LastCommit to be 'commit2', got %q", state.LastCommit)
	}
	if state.LastIndexed != "commit2" {
		t.Errorf("Expected LastIndexed to be 'commit2', got %q", state.LastIndexed)
	}
}

func TestService_IncrementalIndex_ExactBoundary(t *testing.T) {
	dir := t.TempDir()
	settings := &config.GitReposSettings{
		URLs:        []string{"git@github.com:test/repo.git"},
		BaseDir:     dir,
		SyncTimeout: 5 * time.Second,
		MaxFileSize: 256 * 1024,
	}

	svc, err := NewService(settings)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	defer func() {
		if err := svc.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	repoID := "github.com_test_repo"
	repoDir := filepath.Join(dir, "repos", repoID)
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("Failed to create repo dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "main.go"), []byte("package main"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	mock := NewMockExecutor()
	mock.AddResponse("git clone", []byte{}, nil)
	mock.AddResponse("git rev-parse", []byte("commit1\n"), nil)
	svc.git = NewGitClientWithExecutor(mock)

	ctx := context.Background()
	_ = svc.SyncAll(ctx)

	var diffLines strings.Builder
	for i := 0; i < 100; i++ {
		fmt.Fprintf(&diffLines, "file%d.go\n", i)
	}

	mock2 := NewMockExecutor()
	mock2.AddResponse("git fetch", []byte{}, nil)
	mock2.AddResponse("git rev-parse", []byte("commit2\n"), nil)
	mock2.AddResponse("git reset", []byte{}, nil)
	mock2.AddResponse("git diff", []byte(diffLines.String()), nil)
	svc.git = NewGitClientWithExecutor(mock2)

	manifest := svc.manifest.(*Manifest)
	manifest.SetRepoState(repoID, RepoState{
		URL:         "git@github.com:test/repo.git",
		ClonedAt:    time.Now().Add(-1 * time.Hour),
		LastCommit:  "commit1",
		LastIndexed: "commit1",
		FileCount:   1,
	})

	_ = svc.SyncAll(ctx)

	state := manifest.GetRepoState(repoID)
	if state.LastCommit != "commit2" {
		t.Errorf("Expected LastCommit to be 'commit2', got %q", state.LastCommit)
	}
	if state.LastIndexed != "commit2" {
		t.Errorf("Expected LastIndexed to be 'commit2', got %q", state.LastIndexed)
	}
}

func TestService_IncrementalIndex_DiffError(t *testing.T) {
	dir := t.TempDir()
	settings := &config.GitReposSettings{
		URLs:        []string{"git@github.com:test/repo.git"},
		BaseDir:     dir,
		SyncTimeout: 5 * time.Second,
		MaxFileSize: 256 * 1024,
	}

	svc, err := NewService(settings)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	defer func() {
		if err := svc.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	repoID := "github.com_test_repo"
	repoDir := filepath.Join(dir, "repos", repoID)
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("Failed to create repo dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "main.go"), []byte("package main"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	mock := NewMockExecutor()
	mock.AddResponse("git clone", []byte{}, nil)
	mock.AddResponse("git rev-parse", []byte("commit1\n"), nil)
	svc.git = NewGitClientWithExecutor(mock)

	ctx := context.Background()
	_ = svc.SyncAll(ctx)

	mock2 := NewMockExecutor()
	mock2.AddResponse("git fetch", []byte{}, nil)
	mock2.AddResponse("git rev-parse", []byte("commit2\n"), nil)
	mock2.AddResponse("git reset", []byte{}, nil)
	mock2.AddResponse("git diff", []byte{}, fmt.Errorf("diff failed"))
	svc.git = NewGitClientWithExecutor(mock2)

	manifest := svc.manifest.(*Manifest)
	manifest.SetRepoState(repoID, RepoState{
		URL:         "git@github.com:test/repo.git",
		ClonedAt:    time.Now().Add(-1 * time.Hour),
		LastCommit:  "commit1",
		LastIndexed: "commit1",
		FileCount:   1,
	})

	_ = svc.SyncAll(ctx)

	state := manifest.GetRepoState(repoID)
	if state.LastCommit != "commit2" {
		t.Errorf("Expected LastCommit to be 'commit2', got %q", state.LastCommit)
	}
	if state.LastIndexed != "commit2" {
		t.Errorf("Expected LastIndexed to be 'commit2', got %q", state.LastIndexed)
	}
}

func TestService_Initialize_FollowerPath(t *testing.T) {
	dir := t.TempDir()
	settings := &config.GitReposSettings{
		URLs:        []string{"git@github.com:test/repo.git"},
		BaseDir:     dir,
		SyncTimeout: 2 * time.Second,
		MaxFileSize: 256 * 1024,
	}

	// Pre-acquire the lock to simulate another process holding it
	lockPath := filepath.Join(dir, "sync.lock")
	leaderLock := NewFileLock(lockPath)
	acquired, err := leaderLock.TryLock()
	if err != nil {
		t.Fatalf("Failed to acquire leader lock: %v", err)
	}
	if !acquired {
		t.Fatal("Expected to acquire leader lock")
	}

	svc, err := NewService(settings)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	defer func() {
		if err := svc.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	mock := NewMockExecutor()
	mock.AddResponse("git clone", []byte{}, nil)
	mock.AddResponse("git rev-parse", []byte("abc123\n"), nil)
	svc.git = NewGitClientWithExecutor(mock)

	repoID := "github.com_test_repo"
	repoDir := filepath.Join(dir, "repos", repoID)
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("Failed to create repo dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "test.go"), []byte("package test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	filter := NewFileFilter(settings.MaxFileSize)
	indexer := NewIndexer(settings.BaseDir, filter, settings.MaxFileSize)
	_, err = indexer.FullIndex(repoID, repoDir)
	if err != nil {
		t.Fatalf("Pre-index failed: %v", err)
	}

	go func() {
		time.Sleep(200 * time.Millisecond)
		if err := leaderLock.Unlock(); err != nil {
			t.Logf("Failed to unlock leader: %v", err)
		}
	}()

	ctx := context.Background()
	err = svc.Initialize(ctx)
	if err != nil {
		t.Logf("Initialize returned error (may be expected): %v", err)
	}

	if !svc.IsReady() {
		t.Error("Service should be ready after follower initialization completed")
	}
}

func TestService_FetchExistingRepo(t *testing.T) {
	dir := t.TempDir()
	settings := &config.GitReposSettings{
		URLs:        []string{"git@github.com:test/repo.git"},
		BaseDir:     dir,
		SyncTimeout: 5 * time.Second,
		MaxFileSize: 256 * 1024,
	}

	svc, err := NewService(settings)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	defer func() {
		if err := svc.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	repoID := "github.com_test_repo"
	repoDir := filepath.Join(dir, "repos", repoID)
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("Failed to create repo dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "main.go"), []byte("package main"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	manifest := svc.manifest.(*Manifest)
	manifest.SetRepoState(repoID, RepoState{
		URL:         "git@github.com:test/repo.git",
		ClonedAt:    time.Now().Add(-1 * time.Hour),
		LastCommit:  "abc123",
		LastIndexed: "abc123",
	})

	mock := NewMockExecutor()
	mock.AddResponse("git fetch", []byte{}, nil)
	mock.AddResponse("git rev-parse", []byte("abc123\n"), nil)
	svc.git = NewGitClientWithExecutor(mock)

	ctx := context.Background()
	err = svc.SyncAll(ctx)
	if err != nil {
		t.Logf("SyncAll error: %v", err)
	}

	calls := mock.GetCalls()
	if len(calls) == 0 {
		t.Fatal("Expected at least one call")
	}
	found := false
	for _, call := range calls {
		if len(call.Args) > 0 && call.Args[0] == "fetch" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected fetch command to be called")
	}
}

func TestService_SkipReindexWhenUnchanged(t *testing.T) {
	dir := t.TempDir()
	settings := &config.GitReposSettings{
		URLs:        []string{"git@github.com:test/repo.git"},
		BaseDir:     dir,
		SyncTimeout: 5 * time.Second,
		MaxFileSize: 256 * 1024,
	}

	svc, err := NewService(settings)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	defer func() {
		if err := svc.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	repoID := "github.com_test_repo"
	repoDir := filepath.Join(dir, "repos", repoID)
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("Failed to create repo dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "main.go"), []byte("package main"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	manifest := svc.manifest.(*Manifest)
	manifest.SetRepoState(repoID, RepoState{
		URL:         "git@github.com:test/repo.git",
		ClonedAt:    time.Now().Add(-1 * time.Hour),
		LastCommit:  "same_commit",
		LastIndexed: "same_commit",
		FileCount:   1,
	})

	mock := NewMockExecutor()
	mock.AddResponse("git fetch", []byte{}, nil)
	mock.AddResponse("git rev-parse", []byte("same_commit\n"), nil)
	svc.git = NewGitClientWithExecutor(mock)

	ctx := context.Background()
	err = svc.SyncAll(ctx)
	if err != nil {
		t.Logf("SyncAll error: %v", err)
	}

	state := manifest.GetRepoState(repoID)
	if state.FileCount != 1 {
		t.Errorf("Expected FileCount to remain 1, got %d", state.FileCount)
	}
}

func TestService_ErrorIsolation(t *testing.T) {
	dir := t.TempDir()
	settings := &config.GitReposSettings{
		URLs: []string{
			"git@github.com:test/good.git",
			"git@github.com:test/bad.git",
		},
		BaseDir:     dir,
		SyncTimeout: 5 * time.Second,
		MaxFileSize: 256 * 1024,
	}

	svc, err := NewService(settings)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	defer func() {
		if err := svc.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	mock := NewMockExecutor()
	mock.AddResponse("git clone --depth 1 --single-branch git@github.com:test/good.git", []byte{}, nil)
	mock.AddResponse("git rev-parse HEAD", []byte("abc123\n"), nil)
	mock.AddResponse("git clone --depth 1 --single-branch git@github.com:test/bad.git", []byte{}, context.DeadlineExceeded)
	svc.git = NewGitClientWithExecutor(mock)

	goodRepoDir := filepath.Join(dir, "repos", "github.com_test_good")
	if err := os.MkdirAll(goodRepoDir, 0755); err != nil {
		t.Fatalf("Failed to create repo dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(goodRepoDir, "main.go"), []byte("package main"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	ctx := context.Background()
	err = svc.SyncAll(ctx)

	if err == nil {
		t.Error("Expected error for failed repo")
	}

	manifest := svc.manifest.(*Manifest)
	errCount := 0
	if state := manifest.GetRepoState("github.com_test_good"); state.Error != "" {
		errCount++
	}
	if state := manifest.GetRepoState("github.com_test_bad"); state.Error != "" {
		errCount++
	}
	if errCount == 0 {
		t.Error("Expected at least one repo to have an error")
	}
}

func TestService_Initialize_NoURLs(t *testing.T) {
	dir := t.TempDir()
	settings := &config.GitReposSettings{
		URLs:        []string{},
		BaseDir:     dir,
		SyncTimeout: 1 * time.Second,
		MaxFileSize: 256 * 1024,
	}

	svc, err := NewService(settings)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	defer func() {
		if err := svc.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	ctx := context.Background()
	if err := svc.Initialize(ctx); err != nil {
		t.Errorf("Initialize with no URLs should succeed: %v", err)
	}

	if svc.IsReady() {
		t.Error("Service should not be ready with no URLs")
	}
}
