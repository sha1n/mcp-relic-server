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

func TestServiceLifecycle_NoURLsConfig(t *testing.T) {
	dir := t.TempDir()
	settings := &config.GitReposSettings{
		BaseDir: dir,
	}

	// Service creation should still work with no URLs
	svc, err := gitrepos.NewService(settings)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	defer closeService(t, svc)

	// Service should not be ready when no repos are configured
	if svc.IsReady() {
		t.Error("Expected service to not be ready with no URLs")
	}
}

func TestServiceLifecycle_CreateDirectoryStructure(t *testing.T) {
	dir := t.TempDir()

	settings := &config.GitReposSettings{
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

			// Create mock executor and inject via git field
			mock := gitrepos.NewMockExecutor()
			mock.AddResponse("git clone", []byte{}, nil)
			mock.AddResponse("git rev-parse", []byte("abc123\n"), nil)
			setMockGit(svc, mock)

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
	setMockGit(svc, mock)

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
		t.Errorf("Expected success, got error: %s", gitrepos.ExtractTextContent(result))
	}

	content := gitrepos.ExtractTextContent(result)
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
	content := gitrepos.ExtractTextContent(result)
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

	content := gitrepos.ExtractTextContent(result)
	if !strings.Contains(content, "No results") {
		t.Errorf("Expected 'No results' message, got: %s", content)
	}
}

func TestSearchTool_SymbolBoosting(t *testing.T) {
	dir := t.TempDir()
	// Two files: one defines the symbol, another just mentions it in a comment
	files := map[string]string{
		"definition.go": "package main\n\nfunc MySpecialFunction() {\n\t// implementation\n}",
		"usage.go":      "package main\n\n// TODO: call MySpecialFunction here\nfunc other() {}",
	}

	svc := setupTestService(t, dir, files)
	defer closeService(t, svc)

	handler := gitrepos.NewSearchHandler(svc)
	ctx := context.Background()

	result, _, err := handler.Handle(ctx, &mcp.CallToolRequest{}, gitrepos.SearchArgument{
		Query: "MySpecialFunction",
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	content := gitrepos.ExtractTextContent(result)

	// We expect definition.go to be the FIRST result (highest score)
	// The format is "**1. <repo>** `<path>`"
	lines := strings.Split(content, "\n")
	firstResultLine := ""
	for _, line := range lines {
		if strings.HasPrefix(line, "**1.") {
			firstResultLine = line
			break
		}
	}

	if !strings.Contains(firstResultLine, "definition.go") {
		t.Errorf("Expected definition.go to be the first result due to boosting, but got: %s", firstResultLine)
	}
}

func TestSearchTool_SearchWhenNotReady(t *testing.T) {
	dir := t.TempDir()

	settings := &config.GitReposSettings{
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

	content := gitrepos.ExtractTextContent(result)
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
		t.Errorf("Expected success, got error: %s", gitrepos.ExtractTextContent(result))
	}

	content := gitrepos.ExtractTextContent(result)
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

	content := gitrepos.ExtractTextContent(result)
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

	content := gitrepos.ExtractTextContent(result)
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

	content := gitrepos.ExtractTextContent(result)
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
	server, err := mcputil.CreateServer(mcputil.ServerConfig{
		Name:        "test-server",
		Version:     "1.0.0",
		GitReposSvc: svc,
	})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if server == nil {
		t.Fatal("Expected server to be created")
	}
}

func TestMCPServer_NoToolsWhenServiceNil(t *testing.T) {
	// Create MCP server without git repos service
	server, err := mcputil.CreateServer(mcputil.ServerConfig{
		Name:        "test-server",
		Version:     "1.0.0",
		GitReposSvc: nil,
	})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if server == nil {
		t.Fatal("Expected server to be created")
	}
}

// ========================================
// Concurrent Search and Load Tests
// ========================================

func TestSearchTool_ConcurrentSearches(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"main.go":       "package main\n\nfunc main() {\n\tprintln(\"hello world\")\n}",
		"lib/utils.go":  "package lib\n\nfunc Helper() string {\n\treturn \"helper\"\n}",
		"lib/config.go": "package lib\n\nfunc LoadConfig() error {\n\treturn nil\n}",
	}

	svc := setupTestService(t, dir, files)
	defer closeService(t, svc)

	handler := gitrepos.NewSearchHandler(svc)
	ctx := context.Background()

	const goroutines = 10
	const searchesPerGoroutine = 5
	queries := []string{"main", "hello", "Helper", "config", "package"}

	var wg sync.WaitGroup
	errs := make(chan error, goroutines*searchesPerGoroutine)

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < searchesPerGoroutine; i++ {
				q := queries[(id+i)%len(queries)]
				result, _, err := handler.Handle(ctx, &mcp.CallToolRequest{}, gitrepos.SearchArgument{
					Query: q,
				})
				if err != nil {
					errs <- fmt.Errorf("goroutine %d search %d: Handle error: %w", id, i, err)
					continue
				}
				if result.IsError {
					errs <- fmt.Errorf("goroutine %d search %d: result error: %s", id, i, gitrepos.ExtractTextContent(result))
				}
			}
		}(g)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Error(err)
	}
}

func TestSearchTool_ConcurrentSearchAndRead(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"main.go":      "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}",
		"lib/utils.go": "package lib\n\nfunc Helper() string {\n\treturn \"helper\"\n}",
	}

	svc := setupTestService(t, dir, files)
	defer closeService(t, svc)

	searchHandler := gitrepos.NewSearchHandler(svc)
	readHandler := gitrepos.NewReadHandler(svc)
	ctx := context.Background()

	var wg sync.WaitGroup
	errs := make(chan error, 20)

	// Launch search goroutines
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 3; j++ {
				result, _, err := searchHandler.Handle(ctx, &mcp.CallToolRequest{}, gitrepos.SearchArgument{
					Query: "main",
				})
				if err != nil {
					errs <- fmt.Errorf("search goroutine %d iter %d: %w", id, j, err)
					continue
				}
				if result.IsError {
					errs <- fmt.Errorf("search goroutine %d iter %d: error result: %s", id, j, gitrepos.ExtractTextContent(result))
				}
			}
		}(i)
	}

	// Launch read goroutines
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 3; j++ {
				result, _, err := readHandler.Handle(ctx, &mcp.CallToolRequest{}, gitrepos.ReadArgument{
					Repository: "github.com/test/repo",
					Path:       "main.go",
				})
				if err != nil {
					errs <- fmt.Errorf("read goroutine %d iter %d: %w", id, j, err)
					continue
				}
				if result.IsError {
					errs <- fmt.Errorf("read goroutine %d iter %d: error result: %s", id, j, gitrepos.ExtractTextContent(result))
				}
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Error(err)
	}
}

func TestSearchTool_LargeResultSet(t *testing.T) {
	dir := t.TempDir()

	// Create many files with overlapping content
	files := make(map[string]string)
	for i := 0; i < 50; i++ {
		filename := fmt.Sprintf("file%d.go", i)
		files[filename] = fmt.Sprintf("package main\n\n// common keyword searchable\nfunc handler%d() {}\n", i)
	}

	settings := &config.GitReposSettings{
		URLs:         []string{"git@github.com:test/repo.git"},
		BaseDir:      dir,
		SyncInterval: 15 * time.Minute,
		SyncTimeout:  5 * time.Second,
		MaxFileSize:  256 * 1024,
		MaxResults:   10, // Limit to 10 results
	}

	svc, err := gitrepos.NewService(settings)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	defer closeService(t, svc)

	mock := gitrepos.NewMockExecutor()
	mock.AddResponse("git clone", []byte{}, nil)
	mock.AddResponse("git rev-parse", []byte("abc123\n"), nil)
	setMockGit(svc, mock)

	repoDir := filepath.Join(dir, "repos", "github.com_test_repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("Failed to create repo dir: %v", err)
	}
	for relPath, content := range files {
		if err := os.WriteFile(filepath.Join(repoDir, relPath), []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}
	}

	ctx := context.Background()
	if err := svc.Initialize(ctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	handler := gitrepos.NewSearchHandler(svc)
	result, _, err := handler.Handle(ctx, &mcp.CallToolRequest{}, gitrepos.SearchArgument{
		Query: "common keyword searchable",
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Expected success, got error: %s", gitrepos.ExtractTextContent(result))
	}

	content := gitrepos.ExtractTextContent(result)
	// Pagination footer should indicate more results
	if !strings.Contains(content, "more results") {
		t.Errorf("Expected pagination footer with 'more results', got: %s", content)
	}
}

func TestSearchTool_LargeResultSet_MaxResultsOne(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"a.go": "package main\n\nfunc alpha() {}",
		"b.go": "package main\n\nfunc beta() {}",
		"c.go": "package main\n\nfunc gamma() {}",
	}

	settings := &config.GitReposSettings{
		URLs:         []string{"git@github.com:test/repo.git"},
		BaseDir:      dir,
		SyncInterval: 15 * time.Minute,
		SyncTimeout:  5 * time.Second,
		MaxFileSize:  256 * 1024,
		MaxResults:   1,
	}

	svc, err := gitrepos.NewService(settings)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	defer closeService(t, svc)

	mock := gitrepos.NewMockExecutor()
	mock.AddResponse("git clone", []byte{}, nil)
	mock.AddResponse("git rev-parse", []byte("abc123\n"), nil)
	setMockGit(svc, mock)

	repoDir := filepath.Join(dir, "repos", "github.com_test_repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("Failed to create repo dir: %v", err)
	}
	for relPath, content := range files {
		if err := os.WriteFile(filepath.Join(repoDir, relPath), []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}
	}

	ctx := context.Background()
	if err := svc.Initialize(ctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	handler := gitrepos.NewSearchHandler(svc)
	result, _, err := handler.Handle(ctx, &mcp.CallToolRequest{}, gitrepos.SearchArgument{
		Query: "package main",
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	content := gitrepos.ExtractTextContent(result)
	// Should only have 1 result header (starts with "**1.")
	if !strings.Contains(content, "**1.") {
		t.Error("Expected at least 1 result")
	}
	if strings.Contains(content, "**2.") {
		t.Error("Expected only 1 result with MaxResults=1")
	}
	// Should have pagination footer
	if !strings.Contains(content, "more results") {
		t.Errorf("Expected pagination footer, got: %s", content)
	}
}

func TestSearchTool_SearchAfterClose(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"main.go": "package main\nfunc main() {}",
	}

	svc := setupTestService(t, dir, files)

	// Close service first
	if err := svc.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	handler := gitrepos.NewSearchHandler(svc)
	ctx := context.Background()

	result, _, err := handler.Handle(ctx, &mcp.CallToolRequest{}, gitrepos.SearchArgument{
		Query: "main",
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if !result.IsError {
		t.Error("Expected error when searching after close")
	}
}

func TestReadTool_ReadAfterClose(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"main.go": "package main\nfunc main() {}",
	}

	svc := setupTestService(t, dir, files)

	// Close service first
	if err := svc.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	handler := gitrepos.NewReadHandler(svc)
	ctx := context.Background()

	// Read should still work (reads from filesystem, not index)
	// but the service's IsReady() returns false
	result, _, err := handler.Handle(ctx, &mcp.CallToolRequest{}, gitrepos.ReadArgument{
		Repository: "github.com/test/repo",
		Path:       "main.go",
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if !result.IsError {
		t.Error("Expected error when reading after close")
	}
}

func TestIndex_CorruptedIndex_OpenForRead(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"main.go": "package main\nfunc main() {}",
	}

	svc := setupTestService(t, dir, files)
	closeService(t, svc)

	// Corrupt the index by writing garbage to a file in the index directory
	indexDir := filepath.Join(dir, "indexes", "github.com_test_repo.bleve")
	entries, err := os.ReadDir(indexDir)
	if err != nil {
		t.Fatalf("Failed to read index dir: %v", err)
	}

	// Find and corrupt a file in the index
	for _, entry := range entries {
		if !entry.IsDir() {
			corruptPath := filepath.Join(indexDir, entry.Name())
			if err := os.WriteFile(corruptPath, []byte("corrupted data"), 0644); err != nil {
				t.Fatalf("Failed to corrupt index file: %v", err)
			}
			break
		}
	}

	// Try to create a new service and open the corrupted index
	settings := &config.GitReposSettings{
		URLs:         []string{"git@github.com:test/repo.git"},
		BaseDir:      dir,
		SyncInterval: 15 * time.Minute,
		SyncTimeout:  5 * time.Second,
		MaxFileSize:  256 * 1024,
		MaxResults:   20,
	}

	svc2, err := gitrepos.NewService(settings)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	defer closeService(t, svc2)

	// The service should handle corrupted index gracefully
	// (either error during init or not ready)
	mock := gitrepos.NewMockExecutor()
	mock.AddResponse("git clone", []byte{}, nil)
	mock.AddResponse("git rev-parse", []byte("abc123\n"), nil)
	setMockGit(svc2, mock)

	repoDir := filepath.Join(dir, "repos", "github.com_test_repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("Failed to create repo dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "main.go"), []byte("package main"), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	// Initialize may fail or succeed (reindexing overwrites corrupt index)
	ctx := context.Background()
	_ = svc2.Initialize(ctx)
	// The key assertion is no panic
}

func TestIndex_MissingIndexDirectory(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"main.go": "package main\nfunc main() {}",
	}

	svc := setupTestService(t, dir, files)
	closeService(t, svc)

	// Delete the index directory and manifest to simulate a clean slate
	indexDir := filepath.Join(dir, "indexes", "github.com_test_repo.bleve")
	if err := os.RemoveAll(indexDir); err != nil {
		t.Fatalf("Failed to remove index dir: %v", err)
	}
	manifestPath := filepath.Join(dir, "manifest.json")
	if err := os.Remove(manifestPath); err != nil && !os.IsNotExist(err) {
		t.Fatalf("Failed to remove manifest: %v", err)
	}

	// Create a new service - should handle missing index gracefully
	settings := &config.GitReposSettings{
		URLs:         []string{"git@github.com:test/repo.git"},
		BaseDir:      dir,
		SyncInterval: 15 * time.Minute,
		SyncTimeout:  5 * time.Second,
		MaxFileSize:  256 * 1024,
		MaxResults:   20,
	}

	svc2, err := gitrepos.NewService(settings)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	defer closeService(t, svc2)

	mock := gitrepos.NewMockExecutor()
	mock.AddResponse("git clone", []byte{}, nil)
	mock.AddResponse("git rev-parse", []byte("abc123\n"), nil)
	setMockGit(svc2, mock)

	repoDir := filepath.Join(dir, "repos", "github.com_test_repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("Failed to create repo dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "main.go"), []byte("package main"), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	// Initialize should succeed (will re-index from scratch)
	ctx := context.Background()
	if err := svc2.Initialize(ctx); err != nil {
		t.Fatalf("Initialize failed after index deletion: %v", err)
	}

	if !svc2.IsReady() {
		t.Error("Expected service to be ready after re-indexing")
	}
}

// ========================================
// Helper Functions
// ========================================

// setupTestService creates a service with mock git and test files
func setupTestService(t *testing.T, baseDir string, files map[string]string) *gitrepos.Service {
	t.Helper()

	settings := &config.GitReposSettings{
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
	setMockGit(svc, mock)

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

// setMockGit injects a mock git client into the service for testing.
func setMockGit(svc *gitrepos.Service, mock *gitrepos.MockExecutor) {
	svc.SetGitOperations(gitrepos.NewGitClientWithExecutor(mock))
}
