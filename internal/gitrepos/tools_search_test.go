package gitrepos

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sha1n/mcp-relic-server/internal/config"
)

func TestNewSearchHandler(t *testing.T) {
	dir := t.TempDir()
	settings := &config.GitReposSettings{
		Enabled:     true,
		BaseDir:     dir,
		MaxFileSize: 256 * 1024,
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

	handler := NewSearchHandler(svc)
	if handler == nil {
		t.Fatal("Expected non-nil handler")
	}
}

func TestSearchHandler_NotReady(t *testing.T) {
	dir := t.TempDir()
	settings := &config.GitReposSettings{
		Enabled:     true,
		BaseDir:     dir,
		MaxFileSize: 256 * 1024,
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

	handler := NewSearchHandler(svc)
	ctx := context.Background()

	result, _, err := handler.Handle(ctx, &mcp.CallToolRequest{}, SearchArgument{Query: "test"})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if !result.IsError {
		t.Error("Expected error result when service not ready")
	}
}

func TestSearchHandler_EmptyQuery(t *testing.T) {
	dir := t.TempDir()
	svc := setupSearchService(t, dir, nil)
	defer func() {
		if err := svc.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	handler := NewSearchHandler(svc)
	ctx := context.Background()

	result, _, err := handler.Handle(ctx, &mcp.CallToolRequest{}, SearchArgument{Query: ""})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if !result.IsError {
		t.Error("Expected error result for empty query")
	}
}

func TestSearchHandler_SimpleSearch(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"main.go":      "package main\n\nfunc main() {\n\tprintln(\"hello world\")\n}",
		"lib/utils.go": "package lib\n\nfunc Helper() string {\n\treturn \"helper\"\n}",
	}
	svc := setupSearchService(t, dir, files)
	defer func() {
		if err := svc.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	handler := NewSearchHandler(svc)
	ctx := context.Background()

	result, _, err := handler.Handle(ctx, &mcp.CallToolRequest{}, SearchArgument{Query: "hello"})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if result.IsError {
		// Extract actual error text
		errText := ""
		for _, c := range result.Content {
			if tc, ok := c.(*mcp.TextContent); ok {
				errText += tc.Text
			}
		}
		t.Errorf("Expected success, got error: %s", errText)
	}

	// Check that result contains content
	if len(result.Content) == 0 {
		t.Error("Expected content in result")
	}
}

func TestSearchHandler_SearchWithRepositoryFilter(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"main.go": "package main\nfunc main() {}",
	}
	svc := setupSearchService(t, dir, files)
	defer func() {
		if err := svc.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	handler := NewSearchHandler(svc)
	ctx := context.Background()

	// Search with matching repo
	result, _, err := handler.Handle(ctx, &mcp.CallToolRequest{}, SearchArgument{
		Query:      "main",
		Repository: "github.com/test/repo",
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if result.IsError {
		t.Errorf("Expected success, got error")
	}

	// Search with non-matching repo
	result, _, err = handler.Handle(ctx, &mcp.CallToolRequest{}, SearchArgument{
		Query:      "main",
		Repository: "github.com/other/repo",
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	// Should return no results but not an error
	if result.IsError {
		t.Errorf("Expected success (no results), got error")
	}
}

func TestSearchHandler_SearchWithExtensionFilter(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"main.go": "package main\nfunc main() {}",
		"app.py":  "def main():\n    pass",
	}
	svc := setupSearchService(t, dir, files)
	defer func() {
		if err := svc.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	handler := NewSearchHandler(svc)
	ctx := context.Background()

	// Search for "main" with .go extension
	result, _, err := handler.Handle(ctx, &mcp.CallToolRequest{}, SearchArgument{
		Query:     "main",
		Extension: "go",
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if result.IsError {
		t.Errorf("Expected success")
	}

	// Search for "main" with .py extension
	result, _, err = handler.Handle(ctx, &mcp.CallToolRequest{}, SearchArgument{
		Query:     "main",
		Extension: ".py", // With dot prefix
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if result.IsError {
		t.Errorf("Expected success")
	}
}

func TestSearchHandler_SearchWithBothFilters(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"main.go": "package main\nfunc main() {}",
	}
	svc := setupSearchService(t, dir, files)
	defer func() {
		if err := svc.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	handler := NewSearchHandler(svc)
	ctx := context.Background()

	result, _, err := handler.Handle(ctx, &mcp.CallToolRequest{}, SearchArgument{
		Query:      "main",
		Repository: "github.com/test/repo",
		Extension:  "go",
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if result.IsError {
		t.Errorf("Expected success")
	}
}

func TestSearchHandler_NoResults(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"main.go": "package main\nfunc main() {}",
	}
	svc := setupSearchService(t, dir, files)
	defer func() {
		if err := svc.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	handler := NewSearchHandler(svc)
	ctx := context.Background()

	result, _, err := handler.Handle(ctx, &mcp.CallToolRequest{}, SearchArgument{
		Query: "nonexistentterm12345",
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	// Should not be an error, just no results
	if result.IsError {
		t.Errorf("Expected success (no results message), got error")
	}

	// Should contain "No results" message
	if len(result.Content) == 0 {
		t.Error("Expected content")
	}
}

func TestSearchHandler_GetToolDefinition(t *testing.T) {
	dir := t.TempDir()
	settings := &config.GitReposSettings{
		Enabled:     true,
		BaseDir:     dir,
		MaxFileSize: 256 * 1024,
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

	handler := NewSearchHandler(svc)
	tool := handler.GetToolDefinition()

	if tool.Name != "search_code" {
		t.Errorf("Tool name = %q, want 'search_code'", tool.Name)
	}

	if tool.Description == "" {
		t.Error("Tool description should not be empty")
	}
}

func TestSearchHandler_MaxResults(t *testing.T) {
	dir := t.TempDir()
	// Create many files to test max results
	files := make(map[string]string)
	for i := 0; i < 30; i++ {
		files[fmt.Sprintf("file%d.go", i)] = fmt.Sprintf("package pkg%d\nfunc Func%d() {}", i, i)
	}
	svc := setupSearchServiceWithMaxResults(t, dir, files, 5)
	defer func() {
		if err := svc.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	handler := NewSearchHandler(svc)
	ctx := context.Background()

	result, _, err := handler.Handle(ctx, &mcp.CallToolRequest{}, SearchArgument{
		Query: "package",
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if result.IsError {
		t.Errorf("Expected success")
	}

	// Result should mention there are more results
	// (we indexed 30 files but MaxResults is 5)
}

// Helper to set up a service with indexed files for testing
func setupSearchService(t *testing.T, baseDir string, files map[string]string) *Service {
	t.Helper()
	return setupSearchServiceWithMaxResults(t, baseDir, files, 20)
}

func setupSearchServiceWithMaxResults(t *testing.T, baseDir string, files map[string]string, maxResults int) *Service {
	t.Helper()

	settings := &config.GitReposSettings{
		Enabled:     true,
		URLs:        []string{"git@github.com:test/repo.git"},
		BaseDir:     baseDir,
		SyncTimeout: 5 * time.Second,
		MaxFileSize: 256 * 1024,
		MaxResults:  maxResults,
	}

	svc, err := NewService(settings)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	// Create mock executor
	mock := NewMockExecutor()
	mock.AddResponse("git clone", []byte{}, nil)
	mock.AddResponse("git rev-parse", []byte("abc123\n"), nil)
	svc.git = NewGitClientWithExecutor(mock)

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

	// Initialize to index files
	ctx := context.Background()
	if err := svc.Initialize(ctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	return svc
}
