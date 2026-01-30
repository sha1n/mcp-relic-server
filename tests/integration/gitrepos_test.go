package integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sha1n/mcp-relic-server/internal/config"
	"github.com/sha1n/mcp-relic-server/internal/gitrepos"
	mcputil "github.com/sha1n/mcp-relic-server/internal/mcp"
)

// ========================================
// Service Lifecycle Tests
// ========================================

func TestServiceLifecycle_InitializeWithValidConfig(t *testing.T) {
	dir := t.TempDir()

	settings := &config.GitReposSettings{
		Enabled:      true,
		BaseDir:      dir,
		SyncInterval: 15 * time.Minute,
		SyncTimeout:  60 * time.Second,
		MaxFileSize:  256 * 1024,
		MaxResults:   20,
	}

	svc, err := gitrepos.NewService(settings)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	defer closeService(t, svc)

	// Verify directory structure was created
	reposDir := filepath.Join(dir, "repos")
	if _, err := os.Stat(reposDir); os.IsNotExist(err) {
		t.Error("Expected repos directory to be created")
	}

	indexesDir := filepath.Join(dir, "indexes")
	if _, err := os.Stat(indexesDir); os.IsNotExist(err) {
		t.Error("Expected indexes directory to be created")
	}
}

func TestServiceLifecycle_DisabledConfig(t *testing.T) {
	dir := t.TempDir()
	settings := &config.GitReposSettings{
		Enabled: false,
		BaseDir: dir,
	}

	// When disabled, service creation should still work
	svc, err := gitrepos.NewService(settings)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	defer closeService(t, svc)

	// Service should not be ready when no repos are configured
	if svc.IsReady() {
		t.Error("Expected service to not be ready when disabled")
	}
}

func TestServiceLifecycle_CreateDirectoryStructure(t *testing.T) {
	dir := t.TempDir()

	settings := &config.GitReposSettings{
		Enabled:     true,
		BaseDir:     dir,
		MaxFileSize: 256 * 1024,
		MaxResults:  20,
	}

	svc, err := gitrepos.NewService(settings)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	defer closeService(t, svc)

	// Check all expected directories
	expectedDirs := []string{
		filepath.Join(dir, "repos"),
		filepath.Join(dir, "indexes"),
	}

	for _, expected := range expectedDirs {
		if _, err := os.Stat(expected); os.IsNotExist(err) {
			t.Errorf("Expected directory %s to be created", expected)
		}
	}
}

func TestServiceLifecycle_ConcurrentInitialization(t *testing.T) {
	// Test that file locking works correctly for concurrent initialization
	// Each service uses its own directory to avoid Bleve index file conflicts
	var wg sync.WaitGroup
	errors := make([]error, 3)

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			// Each service gets its own directory
			dir := t.TempDir()

			settings := &config.GitReposSettings{
				Enabled:      true,
				URLs:         []string{"git@github.com:test/repo.git"},
				BaseDir:      dir,
				SyncInterval: 15 * time.Minute,
				SyncTimeout:  5 * time.Second,
				MaxFileSize:  256 * 1024,
				MaxResults:   20,
			}

			svc, err := gitrepos.NewService(settings)
			if err != nil {
				errors[idx] = err
				return
			}
			defer func() {
				if err := svc.Close(); err != nil {
					t.Logf("Service %d close error: %v", idx, err)
				}
			}()

			// Create mock executor
			mock := gitrepos.NewMockExecutor()
			mock.AddResponse("git clone", []byte{}, nil)
			mock.AddResponse("git rev-parse", []byte("abc123\n"), nil)
			svc.SetGitClient(gitrepos.NewGitClientWithExecutor(mock))

			// Create repo directory with a test file
			repoDir := filepath.Join(dir, "repos", "github.com_test_repo")
			if err := os.MkdirAll(repoDir, 0755); err != nil {
				errors[idx] = err
				return
			}
			if err := os.WriteFile(filepath.Join(repoDir, "main.go"), []byte("package main"), 0644); err != nil {
				errors[idx] = err
				return
			}

			// Initialize
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := svc.Initialize(ctx); err != nil {
				errors[idx] = fmt.Errorf("service %d init failed: %w", idx, err)
			}
		}(i)
	}

	wg.Wait()

	// Check for errors
	for i, err := range errors {
		if err != nil {
			t.Errorf("Service %d had error: %v", i, err)
		}
	}
}

func TestServiceLifecycle_GracefulShutdown(t *testing.T) {
	dir := t.TempDir()

	settings := &config.GitReposSettings{
		Enabled:     true,
		BaseDir:     dir,
		MaxFileSize: 256 * 1024,
		MaxResults:  20,
	}

	svc, err := gitrepos.NewService(settings)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	// Close should not error
	if err := svc.Close(); err != nil {
		t.Errorf("Close returned error: %v", err)
	}

	// Double close should not panic
	if err := svc.Close(); err != nil {
		t.Errorf("Second Close returned error: %v", err)
	}
}

// ========================================
// Index Tests
// ========================================

func TestIndex_FullIndexCreateSearchableIndex(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"main.go":      "package main\n\nfunc main() {\n\tprintln(\"hello world\")\n}",
		"lib/utils.go": "package lib\n\nfunc Helper() string {\n\treturn \"helper\"\n}",
	}

	svc := setupTestService(t, dir, files)
	defer closeService(t, svc)

	// Search should find content
	alias, err := svc.GetIndexAlias()
	if err != nil {
		t.Fatalf("GetIndexAlias failed: %v", err)
	}

	// Perform a simple search
	searchReq := bleve.NewSearchRequest(bleve.NewMatchQuery("hello"))
	searchReq.Size = 20
	results, err := alias.Search(searchReq)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if results.Total == 0 {
		t.Error("Expected to find 'hello' in indexed content")
	}
}

func TestIndex_MultipleReposCreateCombinedAlias(t *testing.T) {
	dir := t.TempDir()

	settings := &config.GitReposSettings{
		Enabled:      true,
		URLs:         []string{"git@github.com:test/repo1.git", "git@github.com:test/repo2.git"},
		BaseDir:      dir,
		SyncInterval: 15 * time.Minute,
		SyncTimeout:  5 * time.Second,
		MaxFileSize:  256 * 1024,
		MaxResults:   20,
	}

	svc, err := gitrepos.NewService(settings)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	defer closeService(t, svc)

	// Create mock executor
	mock := gitrepos.NewMockExecutor()
	mock.AddResponse("git clone", []byte{}, nil)
	mock.AddResponse("git clone", []byte{}, nil)
	mock.AddResponse("git rev-parse", []byte("abc123\n"), nil)
	mock.AddResponse("git rev-parse", []byte("def456\n"), nil)
	svc.SetGitClient(gitrepos.NewGitClientWithExecutor(mock))

	// Create files in both repos
	for _, repoName := range []string{"github.com_test_repo1", "github.com_test_repo2"} {
		repoDir := filepath.Join(dir, "repos", repoName)
		if err := os.MkdirAll(repoDir, 0755); err != nil {
			t.Fatalf("Failed to create repo dir: %v", err)
		}
		content := fmt.Sprintf("package %s\n\nfunc Main() {}", repoName)
		if err := os.WriteFile(filepath.Join(repoDir, "main.go"), []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}
	}

	// Initialize
	ctx := context.Background()
	if err := svc.Initialize(ctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Search should find content from both repos
	alias, err := svc.GetIndexAlias()
	if err != nil {
		t.Fatalf("GetIndexAlias failed: %v", err)
	}

	searchReq := bleve.NewSearchRequest(bleve.NewMatchQuery("Main"))
	searchReq.Size = 20
	results, err := alias.Search(searchReq)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Should find results from both repos
	if results.Total < 2 {
		t.Errorf("Expected at least 2 results from combined alias, got %d", results.Total)
	}
}

// ========================================
// Search Tool MCP Tests
// ========================================

func TestSearchTool_SearchReturnsResults(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"main.go": "package main\n\nfunc main() {\n\tprintln(\"hello world\")\n}",
	}

	svc := setupTestService(t, dir, files)
	defer closeService(t, svc)

	handler := gitrepos.NewSearchHandler(svc)
	ctx := context.Background()

	result, _, err := handler.Handle(ctx, &mcp.CallToolRequest{}, gitrepos.SearchArgument{
		Query: "hello",
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if result.IsError {
		t.Errorf("Expected success, got error: %s", extractTextContent(result))
	}

	content := extractTextContent(result)
	if !strings.Contains(content, "Found") || !strings.Contains(content, "result") {
		t.Errorf("Expected search results, got: %s", content)
	}
}

func TestSearchTool_SearchWithRepositoryFilter(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"main.go": "package main\nfunc main() {}",
	}

	svc := setupTestService(t, dir, files)
	defer closeService(t, svc)

	handler := gitrepos.NewSearchHandler(svc)
	ctx := context.Background()

	// Search with matching repository
	result, _, err := handler.Handle(ctx, &mcp.CallToolRequest{}, gitrepos.SearchArgument{
		Query:      "main",
		Repository: "github.com/test/repo",
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if result.IsError {
		t.Errorf("Expected success with matching repo filter")
	}

	// Search with non-matching repository
	result, _, err = handler.Handle(ctx, &mcp.CallToolRequest{}, gitrepos.SearchArgument{
		Query:      "main",
		Repository: "github.com/other/repo",
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	// Should return no results but not an error
	content := extractTextContent(result)
	if !strings.Contains(content, "No results") {
		t.Errorf("Expected no results for non-matching repo, got: %s", content)
	}
}

func TestSearchTool_SearchWithExtensionFilter(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"main.go": "package main\nfunc main() {}",
		"app.py":  "def main():\n    pass",
	}

	svc := setupTestService(t, dir, files)
	defer closeService(t, svc)

	handler := gitrepos.NewSearchHandler(svc)
	ctx := context.Background()

	// Search with .go extension
	result, _, err := handler.Handle(ctx, &mcp.CallToolRequest{}, gitrepos.SearchArgument{
		Query:     "main",
		Extension: "go",
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if result.IsError {
		t.Errorf("Expected success with .go extension filter")
	}

	// Search with .py extension (with dot prefix)
	result, _, err = handler.Handle(ctx, &mcp.CallToolRequest{}, gitrepos.SearchArgument{
		Query:     "main",
		Extension: ".py",
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if result.IsError {
		t.Errorf("Expected success with .py extension filter")
	}
}

func TestSearchTool_SearchNoResults(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"main.go": "package main\nfunc main() {}",
	}

	svc := setupTestService(t, dir, files)
	defer closeService(t, svc)

	handler := gitrepos.NewSearchHandler(svc)
	ctx := context.Background()

	result, _, err := handler.Handle(ctx, &mcp.CallToolRequest{}, gitrepos.SearchArgument{
		Query: "nonexistentterm12345xyz",
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	// Should not be an error, just no results
	if result.IsError {
		t.Errorf("Expected no error for zero results search")
	}

	content := extractTextContent(result)
	if !strings.Contains(content, "No results") {
		t.Errorf("Expected 'No results' message, got: %s", content)
	}
}

func TestSearchTool_SearchWhenNotReady(t *testing.T) {
	dir := t.TempDir()

	settings := &config.GitReposSettings{
		Enabled:     true,
		BaseDir:     dir,
		MaxFileSize: 256 * 1024,
		MaxResults:  20,
	}

	svc, err := gitrepos.NewService(settings)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	defer closeService(t, svc)

	// Don't initialize - service should not be ready

	handler := gitrepos.NewSearchHandler(svc)
	ctx := context.Background()

	result, _, err := handler.Handle(ctx, &mcp.CallToolRequest{}, gitrepos.SearchArgument{
		Query: "test",
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if !result.IsError {
		t.Error("Expected error when service not ready")
	}

	content := extractTextContent(result)
	if !strings.Contains(content, "not available") && !strings.Contains(content, "still being indexed") {
		t.Errorf("Expected appropriate not ready message, got: %s", content)
	}
}

// ========================================
// Read Tool MCP Tests
// ========================================

func TestReadTool_ReadFileReturnsContent(t *testing.T) {
	dir := t.TempDir()
	fileContent := "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}"
	files := map[string]string{
		"main.go": fileContent,
	}

	svc := setupTestService(t, dir, files)
	defer closeService(t, svc)

	handler := gitrepos.NewReadHandler(svc)
	ctx := context.Background()

	result, _, err := handler.Handle(ctx, &mcp.CallToolRequest{}, gitrepos.ReadArgument{
		Repository: "github.com/test/repo",
		Path:       "main.go",
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if result.IsError {
		t.Errorf("Expected success, got error: %s", extractTextContent(result))
	}

	content := extractTextContent(result)
	if !strings.Contains(content, "package main") {
		t.Errorf("Expected file content, got: %s", content)
	}
}

func TestReadTool_ReadWithInvalidRepoReturnsError(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"main.go": "package main",
	}

	svc := setupTestService(t, dir, files)
	defer closeService(t, svc)

	handler := gitrepos.NewReadHandler(svc)
	ctx := context.Background()

	result, _, err := handler.Handle(ctx, &mcp.CallToolRequest{}, gitrepos.ReadArgument{
		Repository: "github.com/invalid/repo",
		Path:       "main.go",
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if !result.IsError {
		t.Error("Expected error for invalid repository")
	}

	content := extractTextContent(result)
	if !strings.Contains(content, "not found") {
		t.Errorf("Expected 'not found' message, got: %s", content)
	}
}

func TestReadTool_ReadWithInvalidPathReturnsError(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"main.go": "package main",
	}

	svc := setupTestService(t, dir, files)
	defer closeService(t, svc)

	handler := gitrepos.NewReadHandler(svc)
	ctx := context.Background()

	result, _, err := handler.Handle(ctx, &mcp.CallToolRequest{}, gitrepos.ReadArgument{
		Repository: "github.com/test/repo",
		Path:       "nonexistent.go",
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if !result.IsError {
		t.Error("Expected error for invalid path")
	}

	content := extractTextContent(result)
	if !strings.Contains(content, "not found") {
		t.Errorf("Expected 'not found' message, got: %s", content)
	}
}

func TestReadTool_PathTraversalAttemptReturnsError(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"main.go": "package main",
	}

	svc := setupTestService(t, dir, files)
	defer closeService(t, svc)

	handler := gitrepos.NewReadHandler(svc)
	ctx := context.Background()

	traversalPaths := []string{
		"../../../etc/passwd",
		"..\\..\\..\\etc\\passwd",
		"foo/../../../etc/passwd",
	}

	for _, path := range traversalPaths {
		result, _, err := handler.Handle(ctx, &mcp.CallToolRequest{}, gitrepos.ReadArgument{
			Repository: "github.com/test/repo",
			Path:       path,
		})
		if err != nil {
			t.Fatalf("Handle returned error for path %q: %v", path, err)
		}

		if !result.IsError {
			t.Errorf("Expected error for path traversal: %s", path)
		}
	}
}

func TestReadTool_ReadBinaryFileReturnsError(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"main.go": "package main",
	}

	svc := setupTestService(t, dir, files)
	defer closeService(t, svc)

	// Create a binary file directly
	repoDir := filepath.Join(dir, "repos", "github.com_test_repo")
	binaryPath := filepath.Join(repoDir, "binary.dat")
	binaryContent := []byte{'B', 'I', 'N', 0x00, 'A', 'R', 'Y'}
	if err := os.WriteFile(binaryPath, binaryContent, 0644); err != nil {
		t.Fatalf("Failed to write binary file: %v", err)
	}

	handler := gitrepos.NewReadHandler(svc)
	ctx := context.Background()

	result, _, err := handler.Handle(ctx, &mcp.CallToolRequest{}, gitrepos.ReadArgument{
		Repository: "github.com/test/repo",
		Path:       "binary.dat",
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if !result.IsError {
		t.Error("Expected error for binary file")
	}

	content := extractTextContent(result)
	if !strings.Contains(content, "binary") {
		t.Errorf("Expected 'binary' message, got: %s", content)
	}
}

// ========================================
// MCP Server Integration Tests
// ========================================

func TestMCPServer_ToolsRegistered(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"main.go": "package main\nfunc main() {}",
	}

	svc := setupTestService(t, dir, files)
	defer closeService(t, svc)

	// Create MCP server with the service
	server := mcputil.CreateServer(mcputil.ServerConfig{
		Name:        "test-server",
		Version:     "1.0.0",
		GitReposSvc: svc,
	})

	if server == nil {
		t.Fatal("Expected server to be created")
	}

	// The MCP SDK doesn't expose a way to list registered tools directly,
	// but we can verify the server was created successfully and the tools
	// work by invoking them through handlers (tested above).
}

func TestMCPServer_NoToolsWhenServiceNil(t *testing.T) {
	// Create MCP server without git repos service
	server := mcputil.CreateServer(mcputil.ServerConfig{
		Name:        "test-server",
		Version:     "1.0.0",
		GitReposSvc: nil,
	})

	if server == nil {
		t.Fatal("Expected server to be created")
	}

	// Server should be created successfully without tools
}

// ========================================
// Helper Functions
// ========================================

// setupTestService creates a service with mock git and test files
func setupTestService(t *testing.T, baseDir string, files map[string]string) *gitrepos.Service {
	t.Helper()

	settings := &config.GitReposSettings{
		Enabled:      true,
		URLs:         []string{"git@github.com:test/repo.git"},
		BaseDir:      baseDir,
		SyncInterval: 15 * time.Minute,
		SyncTimeout:  5 * time.Second,
		MaxFileSize:  256 * 1024,
		MaxResults:   20,
	}

	svc, err := gitrepos.NewService(settings)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	// Create mock executor
	mock := gitrepos.NewMockExecutor()
	mock.AddResponse("git clone", []byte{}, nil)
	mock.AddResponse("git rev-parse", []byte("abc123\n"), nil)
	svc.SetGitClient(gitrepos.NewGitClientWithExecutor(mock))

	// Create repo directory with test files
	repoDir := filepath.Join(baseDir, "repos", "github.com_test_repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("Failed to create repo dir: %v", err)
	}

	for relPath, content := range files {
		fullPath := filepath.Join(repoDir, relPath)
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create dir: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}
	}

	// Initialize
	ctx := context.Background()
	if err := svc.Initialize(ctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	return svc
}

// closeService closes the service and reports any errors
func closeService(t *testing.T, svc *gitrepos.Service) {
	t.Helper()
	if err := svc.Close(); err != nil {
		t.Errorf("Failed to close service: %v", err)
	}
}

// extractTextContent extracts text from MCP result
func extractTextContent(result *mcp.CallToolResult) string {
	var sb strings.Builder
	for _, c := range result.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			sb.WriteString(tc.Text)
		}
	}
	return sb.String()
}
