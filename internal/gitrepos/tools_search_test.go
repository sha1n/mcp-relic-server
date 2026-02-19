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
// Mock-based handler tests
// ============================

func TestNewSearchHandler(t *testing.T) {
	handler := NewSearchHandler(&mockSearchService{})
	if handler == nil {
		t.Fatal("Expected non-nil handler")
	}
}

func TestSearchHandler_NotReady(t *testing.T) {
	handler := NewSearchHandler(&mockSearchService{ready: false})
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
	handler := NewSearchHandler(&mockSearchService{ready: true})
	ctx := context.Background()

	result, _, err := handler.Handle(ctx, &mcp.CallToolRequest{}, SearchArgument{Query: ""})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected error result for empty query")
	}
}

func TestSearchHandler_WhitespaceOnlyQuery(t *testing.T) {
	handler := NewSearchHandler(&mockSearchService{ready: true})
	ctx := context.Background()

	queries := []string{"   ", "\t", "\n", " \t\n "}
	for _, q := range queries {
		result, _, err := handler.Handle(ctx, &mcp.CallToolRequest{}, SearchArgument{Query: q})
		if err != nil {
			t.Fatalf("Handle returned error for query %q: %v", q, err)
		}
		if !result.IsError {
			t.Errorf("Expected error result for whitespace-only query %q", q)
		}
	}
}

func TestSearchHandler_AliasError(t *testing.T) {
	handler := NewSearchHandler(&mockSearchService{
		ready:    true,
		aliasErr: fmt.Errorf("indexes not ready"),
	})
	ctx := context.Background()

	result, _, err := handler.Handle(ctx, &mcp.CallToolRequest{}, SearchArgument{Query: "test"})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected error result when alias fails")
	}
	content := ExtractTextContent(result)
	if !strings.Contains(content, "Failed to access indexes") {
		t.Errorf("Expected 'Failed to access indexes' message, got: %s", content)
	}
}

func TestSearchHandler_GetToolDefinition(t *testing.T) {
	handler := NewSearchHandler(&mockSearchService{})
	tool := handler.GetToolDefinition()

	if tool.Name != "search" {
		t.Errorf("Tool name = %q, want 'search'", tool.Name)
	}
	if tool.Description == "" {
		t.Error("Tool description should not be empty")
	}
}

func TestSearchHandler_ToolDescriptionContent(t *testing.T) {
	handler := NewSearchHandler(&mockSearchService{})
	tool := handler.GetToolDefinition()

	if !strings.Contains(tool.Description, "WHEN TO USE") {
		t.Error("Tool description should contain 'WHEN TO USE' section")
	}
	if !strings.Contains(tool.Description, "HOW IT WORKS") {
		t.Error("Tool description should contain 'HOW IT WORKS' section")
	}
}

// ============================
// Bleve-based search tests (require real index)
// ============================

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
		errText := ExtractTextContent(result)
		t.Errorf("Expected success, got error: %s", errText)
	}
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
	if result.IsError {
		t.Errorf("Expected success (no results message), got error")
	}
	if len(result.Content) == 0 {
		t.Error("Expected content")
	}
}

func TestSearchHandler_MaxResults(t *testing.T) {
	dir := t.TempDir()
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

	content := ExtractTextContent(result)
	if !strings.Contains(content, "more results") {
		t.Errorf("Expected 'more results' footer in output, got: %s", content)
	}
}

func TestSearchHandler_ResultFormat(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"main.go": "package main\n\nfunc main() {\n\tprintln(\"hello world\")\n}",
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
		t.Fatalf("Expected success, got error")
	}

	content := ExtractTextContent(result)

	if !strings.Contains(content, "**1.") {
		t.Errorf("Expected numbered result header '**1.' in output, got: %s", content)
	}
	if !strings.Contains(content, "github.com/test/repo") {
		t.Errorf("Expected repository name in output, got: %s", content)
	}
	if !strings.Contains(content, "`main.go`") {
		t.Errorf("Expected file path in backticks in output, got: %s", content)
	}
	if !strings.Contains(content, "```go") {
		t.Errorf("Expected language-specific code fence '```go' in output, got: %s", content)
	}
	if !strings.Contains(content, "Found") {
		t.Errorf("Expected 'Found' header in output, got: %s", content)
	}
}

func TestSearchHandler_SubstringRepoFilter(t *testing.T) {
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
		Repository: "test/repo",
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	content := ExtractTextContent(result)
	if result.IsError {
		t.Errorf("Expected success with substring repo filter, got error: %s", content)
	}
}

// ============================
// Helper to set up a service with indexed files for testing
// ============================

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

	mock := NewMockExecutor()
	mock.AddResponse("git clone", []byte{}, nil)
	mock.AddResponse("git rev-parse", []byte("abc123\n"), nil)
	svc.git = NewGitClientWithExecutor(mock)

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

	ctx := context.Background()
	if err := svc.Initialize(ctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	return svc
}
