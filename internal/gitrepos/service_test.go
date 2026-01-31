package gitrepos

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sha1n/mcp-relic-server/internal/config"
)

func TestNewService(t *testing.T) {
	dir := t.TempDir()
	settings := &config.GitReposSettings{
		Enabled:      true,
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
		Enabled:     true,
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

	// Verify nested directories were created
	if _, err := os.Stat(baseDir); err != nil {
		t.Errorf("Base directory should exist: %v", err)
	}
}

func TestService_IsReady_InitiallyFalse(t *testing.T) {
	dir := t.TempDir()
	settings := &config.GitReposSettings{
		Enabled:     true,
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
		Enabled:     true,
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
		Enabled:     true,
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

func TestService_GetSettings(t *testing.T) {
	dir := t.TempDir()
	settings := &config.GitReposSettings{
		Enabled:     true,
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
		Enabled:     true,
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

func TestService_SetGitClient(t *testing.T) {
	svc := &Service{}
	client := NewGitClient()
	svc.SetGitClient(client)
	if svc.git != client {
		t.Error("SetGitClient did not set the client")
	}
}

func TestRegisterTools(t *testing.T) {
	// Minimal mock server to verify registration doesn't panic
	// We can't easily inspect the server's tools without using the MCP SDK internals or integration test.
	// But simply calling them ensures coverage of the function body.
	
	// Since mcp.Server is a struct, we can just instantiate it.
	// But mcp.NewServer requires parameters.
	
	// Using a real mcp.Server for this test introduces a dependency on mcp package which is fine.
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "1.0"}, nil)
	svc := &Service{} // Nil service might panic inside Register if it uses it immediately? 
	// The Register functions passed the service to the handler constructor. 
	// Handler methods check for nil service? No, NewSearchHandler takes *Service.
	// We should pass a valid service.
	
	dir := t.TempDir()
	settings := &config.GitReposSettings{BaseDir: dir}
	svc, _ = NewService(settings)
	
	RegisterSearchTool(server, svc)
	RegisterReadTool(server, svc)
}

// ========================================
// Index Tests
// ========================================

func TestService_SyncAll_NoURLs(t *testing.T) {
	dir := t.TempDir()
	settings := &config.GitReposSettings{
		Enabled:     true,
		URLs:        []string{},
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

	ctx := context.Background()
	if err := svc.SyncAll(ctx); err != nil {
		t.Errorf("SyncAll with no URLs should succeed: %v", err)
	}
}

func TestService_Initialize_NoURLs(t *testing.T) {
	dir := t.TempDir()
	settings := &config.GitReposSettings{
		Enabled:     true,
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

	// Service should not be ready (no indexes)
	if svc.IsReady() {
		t.Error("Service should not be ready with no URLs")
	}
}

func TestService_SyncAll_WithMockGit(t *testing.T) {
	dir := t.TempDir()
	settings := &config.GitReposSettings{
		Enabled:     true,
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

	// Create mock executor that simulates git commands
	mock := NewMockExecutor()

	// Simulate clone
	mock.AddResponse("git clone", []byte{}, nil)
	// Simulate rev-parse for HEAD
	mock.AddResponse("git rev-parse", []byte("abc123def456\n"), nil)

	svc.git = NewGitClientWithExecutor(mock)

	// Create the repo directory structure that would exist after clone
	repoDir := filepath.Join(dir, "repos", "github.com_test_repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("Failed to create repo dir: %v", err)
	}

	// Create a test file to index
	testFile := filepath.Join(repoDir, "main.go")
	if err := os.WriteFile(testFile, []byte("package main\n\nfunc main() {}"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	ctx := context.Background()
	err = svc.SyncAll(ctx)
	// May have error due to mock, but should not panic
	if err != nil {
		t.Logf("SyncAll returned error (expected with mock): %v", err)
	}
}

func TestService_Initialize_LeaderSync(t *testing.T) {
	dir := t.TempDir()
	settings := &config.GitReposSettings{
		Enabled:     true,
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

	// Create mock executor
	mock := NewMockExecutor()
	mock.AddResponse("git clone", []byte{}, nil)
	mock.AddResponse("git rev-parse", []byte("abc123\n"), nil)
	svc.git = NewGitClientWithExecutor(mock)

	// Create repo dir with test file
	repoDir := filepath.Join(dir, "repos", "github.com_test_repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("Failed to create repo dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "test.go"), []byte("package test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	ctx := context.Background()
	err = svc.Initialize(ctx)
	// May have error due to mock, but should complete
	if err != nil {
		t.Logf("Initialize returned error (expected with mock): %v", err)
	}
}

func TestService_RemovesStaleRepos(t *testing.T) {
	dir := t.TempDir()
	settings := &config.GitReposSettings{
		Enabled:     true,
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

	// Add a stale repo to manifest
	svc.manifest.SetRepoState("github.com_old_repo", RepoState{
		URL:      "git@github.com:old/repo.git",
		ClonedAt: time.Now(),
	})

	// Create stale repo directory
	staleRepoDir := filepath.Join(dir, "repos", "github.com_old_repo")
	if err := os.MkdirAll(staleRepoDir, 0755); err != nil {
		t.Fatalf("Failed to create stale repo dir: %v", err)
	}

	// Create mock executor
	mock := NewMockExecutor()
	mock.AddResponse("git clone", []byte{}, nil)
	mock.AddResponse("git rev-parse", []byte("abc123\n"), nil)
	svc.git = NewGitClientWithExecutor(mock)

	// Create repo dir
	repoDir := filepath.Join(dir, "repos", "github.com_test_repo1")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("Failed to create repo dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "test.go"), []byte("package test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	ctx := context.Background()
	_ = svc.SyncAll(ctx) // Ignore errors from mock

	// Check stale repo was removed from manifest
	if svc.manifest.HasRepo("github.com_old_repo") {
		t.Error("Stale repo should have been removed from manifest")
	}

	// Check stale repo directory was removed
	if _, err := os.Stat(staleRepoDir); !os.IsNotExist(err) {
		t.Error("Stale repo directory should have been removed")
	}
}

func TestService_IndexesReadyAfterSync(t *testing.T) {
	dir := t.TempDir()
	settings := &config.GitReposSettings{
		Enabled:     true,
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

	// Create mock executor
	mock := NewMockExecutor()
	mock.AddResponse("git clone", []byte{}, nil)
	mock.AddResponse("git rev-parse", []byte("abc123\n"), nil)
	svc.git = NewGitClientWithExecutor(mock)

	// Create repo dir with test files
	repoDir := filepath.Join(dir, "repos", "github.com_test_repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("Failed to create repo dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "main.go"), []byte("package main\nfunc main() {}"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	ctx := context.Background()
	_ = svc.Initialize(ctx)

	// After initialization with indexed files, service should be ready
	if !svc.IsReady() {
		t.Error("Service should be ready after successful initialization")
	}

	// Should be able to get index alias
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
		Enabled:     true,
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

	// Create mock executor that respects context
	mock := NewMockExecutor()
	mock.AddResponse("git clone", []byte{}, context.Canceled)
	svc.git = NewGitClientWithExecutor(mock)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err = svc.SyncAll(ctx)
	// Should handle cancellation gracefully
	t.Logf("SyncAll with cancelled context: %v", err)
}

func TestService_ParallelSync(t *testing.T) {
	dir := t.TempDir()
	settings := &config.GitReposSettings{
		Enabled: true,
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

	// Create mock executor with responses for all repos
	mock := NewMockExecutor()
	for i := 0; i < 5; i++ {
		mock.AddResponse("git clone", []byte{}, nil)
		mock.AddResponse("git rev-parse", []byte("abc123\n"), nil)
	}
	svc.git = NewGitClientWithExecutor(mock)

	// Create repo dirs with test files
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

	// Should complete without panic
	t.Logf("Parallel sync completed in %v, error: %v", elapsed, err)
}

func TestService_IncrementalIndex(t *testing.T) {
	dir := t.TempDir()
	settings := &config.GitReposSettings{
		Enabled:     true,
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

	// First sync - full index
	mock := NewMockExecutor()
	mock.AddResponse("git clone", []byte{}, nil)
	mock.AddResponse("git rev-parse", []byte("commit1\n"), nil)
	svc.git = NewGitClientWithExecutor(mock)

	ctx := context.Background()
	_ = svc.SyncAll(ctx)

	// Second sync - simulate commit change with file diff
	mock2 := NewMockExecutor()
	mock2.AddResponse("git fetch", []byte{}, nil)
	mock2.AddResponse("git rev-parse", []byte("commit2\n"), nil)
	mock2.AddResponse("git reset", []byte{}, nil)
	mock2.AddResponse("git diff", []byte("main.go\n"), nil)
	svc.git = NewGitClientWithExecutor(mock2)

	// Update manifest to simulate existing state
	svc.manifest.SetRepoState(repoID, RepoState{
		URL:         "git@github.com:test/repo.git",
		ClonedAt:    time.Now().Add(-1 * time.Hour),
		LastCommit:  "commit1",
		LastIndexed: "commit1",
	})

	_ = svc.SyncAll(ctx)

	// Verify manifest was updated
	state := svc.manifest.GetRepoState(repoID)
	if state.LastCommit != "commit2" {
		t.Errorf("Expected LastCommit to be 'commit2', got %q", state.LastCommit)
	}
}

func TestService_FetchExistingRepo(t *testing.T) {
	dir := t.TempDir()
	settings := &config.GitReposSettings{
		Enabled:     true,
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

	// Set up manifest to indicate repo already cloned
	svc.manifest.SetRepoState(repoID, RepoState{
		URL:         "git@github.com:test/repo.git",
		ClonedAt:    time.Now().Add(-1 * time.Hour),
		LastCommit:  "abc123",
		LastIndexed: "abc123",
	})

	// Create mock for fetch (not clone)
	mock := NewMockExecutor()
	mock.AddResponse("git fetch", []byte{}, nil)
	mock.AddResponse("git rev-parse", []byte("abc123\n"), nil) // Same commit
	svc.git = NewGitClientWithExecutor(mock)

	ctx := context.Background()
	err = svc.SyncAll(ctx)
	if err != nil {
		t.Logf("SyncAll error: %v", err)
	}

	// Verify fetch was called (not clone)
	calls := mock.GetCalls()
	if len(calls) == 0 {
		t.Fatal("Expected at least one call")
	}
	// First call should be fetch
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
		Enabled:     true,
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

	// Set up manifest to indicate repo already indexed at current commit
	svc.manifest.SetRepoState(repoID, RepoState{
		URL:         "git@github.com:test/repo.git",
		ClonedAt:    time.Now().Add(-1 * time.Hour),
		LastCommit:  "same_commit",
		LastIndexed: "same_commit",
		FileCount:   1,
	})

	// Create mock
	mock := NewMockExecutor()
	mock.AddResponse("git fetch", []byte{}, nil)
	mock.AddResponse("git rev-parse", []byte("same_commit\n"), nil) // Same commit
	svc.git = NewGitClientWithExecutor(mock)

	ctx := context.Background()
	err = svc.SyncAll(ctx)
	if err != nil {
		t.Logf("SyncAll error: %v", err)
	}

	// FileCount should remain unchanged (no reindex)
	state := svc.manifest.GetRepoState(repoID)
	if state.FileCount != 1 {
		t.Errorf("Expected FileCount to remain 1, got %d", state.FileCount)
	}
}

func TestService_ErrorIsolation(t *testing.T) {
	dir := t.TempDir()
	settings := &config.GitReposSettings{
		Enabled: true,
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

	// Create mock that returns different results based on URL pattern
	// Add enough responses for both repos in any order
	mock := NewMockExecutor()
	// For "good" repo - will succeed
	mock.AddResponse("git clone --depth 1 --single-branch git@github.com:test/good.git", []byte{}, nil)
	mock.AddResponse("git rev-parse HEAD", []byte("abc123\n"), nil)
	// For "bad" repo - will fail
	mock.AddResponse("git clone --depth 1 --single-branch git@github.com:test/bad.git", []byte{}, context.DeadlineExceeded)

	svc.git = NewGitClientWithExecutor(mock)

	// Create repo dir for the successful one
	goodRepoDir := filepath.Join(dir, "repos", "github.com_test_good")
	if err := os.MkdirAll(goodRepoDir, 0755); err != nil {
		t.Fatalf("Failed to create repo dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(goodRepoDir, "main.go"), []byte("package main"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	ctx := context.Background()
	err = svc.SyncAll(ctx)

	// Should return error for failed repo but not crash
	if err == nil {
		t.Error("Expected error for failed repo")
	}

	// At least one repo should have error (the bad one)
	// We can't guarantee which one due to parallel execution, but the test verifies
	// that errors are isolated and don't crash the entire sync
	errCount := 0
	if state := svc.manifest.GetRepoState("github.com_test_good"); state.Error != "" {
		errCount++
	}
	if state := svc.manifest.GetRepoState("github.com_test_bad"); state.Error != "" {
		errCount++
	}
	if errCount == 0 {
		t.Error("Expected at least one repo to have an error")
	}
}
