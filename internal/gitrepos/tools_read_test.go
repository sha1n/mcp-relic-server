package gitrepos

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sha1n/mcp-relic-server/internal/config"
)

func TestNewReadHandler(t *testing.T) {
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

	handler := NewReadHandler(svc)
	if handler == nil {
		t.Fatal("Expected non-nil handler")
	}
}

func TestReadHandler_NotReady(t *testing.T) {
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

	handler := NewReadHandler(svc)
	ctx := context.Background()

	result, _, err := handler.Handle(ctx, &mcp.CallToolRequest{}, ReadArgument{
		Repository: "github.com/test/repo",
		Path:       "main.go",
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if !result.IsError {
		t.Error("Expected error result when service not ready")
	}
}

func TestReadHandler_EmptyRepository(t *testing.T) {
	dir := t.TempDir()
	svc := setupReadService(t, dir, nil)
	defer func() {
		if err := svc.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	handler := NewReadHandler(svc)
	ctx := context.Background()

	result, _, err := handler.Handle(ctx, &mcp.CallToolRequest{}, ReadArgument{
		Repository: "",
		Path:       "main.go",
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if !result.IsError {
		t.Error("Expected error result for empty repository")
	}
}

func TestReadHandler_EmptyPath(t *testing.T) {
	dir := t.TempDir()
	svc := setupReadService(t, dir, nil)
	defer func() {
		if err := svc.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	handler := NewReadHandler(svc)
	ctx := context.Background()

	result, _, err := handler.Handle(ctx, &mcp.CallToolRequest{}, ReadArgument{
		Repository: "github.com/test/repo",
		Path:       "",
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if !result.IsError {
		t.Error("Expected error result for empty path")
	}
}

func TestReadHandler_ReadValidFile(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"main.go": "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}",
	}
	svc := setupReadService(t, dir, files)
	defer func() {
		if err := svc.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	handler := NewReadHandler(svc)
	ctx := context.Background()

	result, _, err := handler.Handle(ctx, &mcp.CallToolRequest{}, ReadArgument{
		Repository: "github.com/test/repo",
		Path:       "main.go",
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if result.IsError {
		errText := extractTextContent(result)
		t.Errorf("Expected success, got error: %s", errText)
	}

	// Check content includes the file content
	content := extractTextContent(result)
	if !strings.Contains(content, "package main") {
		t.Errorf("Expected file content in result, got: %s", content)
	}
}

func TestReadHandler_ReadNestedFile(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"src/lib/utils.go": "package lib\n\nfunc Helper() {}",
	}
	svc := setupReadService(t, dir, files)
	defer func() {
		if err := svc.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	handler := NewReadHandler(svc)
	ctx := context.Background()

	result, _, err := handler.Handle(ctx, &mcp.CallToolRequest{}, ReadArgument{
		Repository: "github.com/test/repo",
		Path:       "src/lib/utils.go",
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if result.IsError {
		errText := extractTextContent(result)
		t.Errorf("Expected success, got error: %s", errText)
	}
}

func TestReadHandler_NonExistentRepository(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"main.go": "package main",
	}
	svc := setupReadService(t, dir, files)
	defer func() {
		if err := svc.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	handler := NewReadHandler(svc)
	ctx := context.Background()

	result, _, err := handler.Handle(ctx, &mcp.CallToolRequest{}, ReadArgument{
		Repository: "github.com/other/repo",
		Path:       "main.go",
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if !result.IsError {
		t.Error("Expected error for non-existent repository")
	}

	content := extractTextContent(result)
	if !strings.Contains(content, "not found") {
		t.Errorf("Expected 'not found' in error, got: %s", content)
	}
}

func TestReadHandler_NonExistentFile(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"main.go": "package main",
	}
	svc := setupReadService(t, dir, files)
	defer func() {
		if err := svc.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	handler := NewReadHandler(svc)
	ctx := context.Background()

	result, _, err := handler.Handle(ctx, &mcp.CallToolRequest{}, ReadArgument{
		Repository: "github.com/test/repo",
		Path:       "nonexistent.go",
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if !result.IsError {
		t.Error("Expected error for non-existent file")
	}

	content := extractTextContent(result)
	if !strings.Contains(content, "not found") {
		t.Errorf("Expected 'not found' in error, got: %s", content)
	}
}

func TestReadHandler_PathTraversalDotDot(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"main.go": "package main",
	}
	svc := setupReadService(t, dir, files)
	defer func() {
		if err := svc.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	handler := NewReadHandler(svc)
	ctx := context.Background()

	testCases := []string{
		"../../../etc/passwd",
		"..\\..\\..\\etc\\passwd",
		"foo/../../../etc/passwd",
		"foo/bar/../../..",
	}

	for _, path := range testCases {
		result, _, err := handler.Handle(ctx, &mcp.CallToolRequest{}, ReadArgument{
			Repository: "github.com/test/repo",
			Path:       path,
		})
		if err != nil {
			t.Fatalf("Handle returned error for path %q: %v", path, err)
		}

		if !result.IsError {
			t.Errorf("Expected error for path traversal attempt: %s", path)
		}
	}
}

func TestReadHandler_AbsolutePath(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"main.go": "package main",
	}
	svc := setupReadService(t, dir, files)
	defer func() {
		if err := svc.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	handler := NewReadHandler(svc)
	ctx := context.Background()

	result, _, err := handler.Handle(ctx, &mcp.CallToolRequest{}, ReadArgument{
		Repository: "github.com/test/repo",
		Path:       "/etc/passwd",
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if !result.IsError {
		t.Error("Expected error for absolute path")
	}
}

func TestReadHandler_ReadDirectory(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"src/main.go": "package main",
	}
	svc := setupReadService(t, dir, files)
	defer func() {
		if err := svc.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	handler := NewReadHandler(svc)
	ctx := context.Background()

	result, _, err := handler.Handle(ctx, &mcp.CallToolRequest{}, ReadArgument{
		Repository: "github.com/test/repo",
		Path:       "src",
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if !result.IsError {
		t.Error("Expected error when reading directory")
	}

	content := extractTextContent(result)
	if !strings.Contains(content, "directory") {
		t.Errorf("Expected 'directory' in error, got: %s", content)
	}
}

func TestReadHandler_FileTooLarge(t *testing.T) {
	dir := t.TempDir()
	// Create a file larger than max size
	largeContent := strings.Repeat("x", 1024) // 1KB
	files := map[string]string{
		"large.txt": largeContent,
	}

	settings := &config.GitReposSettings{
		Enabled:     true,
		URLs:        []string{"git@github.com:test/repo.git"},
		BaseDir:     dir,
		SyncTimeout: 5 * time.Second,
		MaxFileSize: 500, // Only 500 bytes allowed
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

	// Create mock executor
	mock := NewMockExecutor()
	mock.AddResponse("git clone", []byte{}, nil)
	mock.AddResponse("git rev-parse", []byte("abc123\n"), nil)
	svc.git = NewGitClientWithExecutor(mock)

	// Create repo directory with test files
	repoDir := filepath.Join(dir, "repos", "github.com_test_repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("Failed to create repo dir: %v", err)
	}

	for relPath, content := range files {
		fullPath := filepath.Join(repoDir, relPath)
		fileDir := filepath.Dir(fullPath)
		if err := os.MkdirAll(fileDir, 0755); err != nil {
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

	handler := NewReadHandler(svc)
	result, _, err := handler.Handle(ctx, &mcp.CallToolRequest{}, ReadArgument{
		Repository: "github.com/test/repo",
		Path:       "large.txt",
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if !result.IsError {
		t.Error("Expected error for file too large")
	}

	content := extractTextContent(result)
	if !strings.Contains(content, "too large") {
		t.Errorf("Expected 'too large' in error, got: %s", content)
	}
}

func TestReadHandler_BinaryFile(t *testing.T) {
	dir := t.TempDir()
	svc := setupReadService(t, dir, nil)
	defer func() {
		if err := svc.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	// Create a binary file directly
	repoDir := filepath.Join(dir, "repos", "github.com_test_repo")
	binaryPath := filepath.Join(repoDir, "binary.dat")
	binaryContent := []byte{'B', 'I', 'N', 0x00, 'A', 'R', 'Y'}
	if err := os.WriteFile(binaryPath, binaryContent, 0644); err != nil {
		t.Fatalf("Failed to write binary file: %v", err)
	}

	handler := NewReadHandler(svc)
	ctx := context.Background()

	result, _, err := handler.Handle(ctx, &mcp.CallToolRequest{}, ReadArgument{
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
		t.Errorf("Expected 'binary' in error, got: %s", content)
	}
}

func TestReadHandler_GetToolDefinition(t *testing.T) {
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

	handler := NewReadHandler(svc)
	tool := handler.GetToolDefinition()

	if tool.Name != "read_code" {
		t.Errorf("Tool name = %q, want 'read_code'", tool.Name)
	}

	if tool.Description == "" {
		t.Error("Tool description should not be empty")
	}
}

func TestExtensionToLanguage(t *testing.T) {
	tests := []struct {
		ext  string
		want string
	}{
		{"go", "go"},
		{"py", "python"},
		{"js", "javascript"},
		{"ts", "typescript"},
		{"java", "java"},
		{"rs", "rust"},
		{"rb", "ruby"},
		{"yaml", "yaml"},
		{"yml", "yaml"},
		{"json", "json"},
		{"md", "markdown"},
		{"GO", "go"},     // Case insensitive
		{"PY", "python"}, // Case insensitive
		{"xyz", "xyz"},   // Unknown extension returns as-is
	}

	for _, tt := range tests {
		got := extensionToLanguage(tt.ext)
		if got != tt.want {
			t.Errorf("extensionToLanguage(%q) = %q, want %q", tt.ext, got, tt.want)
		}
	}
}

func TestValidatePath(t *testing.T) {
	tests := []struct {
		path    string
		wantErr bool
	}{
		{"main.go", false},
		{"src/main.go", false},
		{"src/lib/utils.go", false},
		{"../etc/passwd", true},
		{"foo/../../../etc/passwd", true},
		{"/etc/passwd", true},
		{"..\\..\\etc\\passwd", true},
		{"foo/bar/..", false}, // This is actually valid after cleaning
	}

	for _, tt := range tests {
		err := validatePath(tt.path)
		if (err != nil) != tt.wantErr {
			t.Errorf("validatePath(%q) error = %v, wantErr = %v", tt.path, err, tt.wantErr)
		}
	}
}

// Helper to set up a service with files for testing
func setupReadService(t *testing.T, baseDir string, files map[string]string) *Service {
	t.Helper()

	settings := &config.GitReposSettings{
		Enabled:     true,
		URLs:        []string{"git@github.com:test/repo.git"},
		BaseDir:     baseDir,
		SyncTimeout: 5 * time.Second,
		MaxFileSize: 256 * 1024,
		MaxResults:  20,
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

// Helper to extract text content from result
func extractTextContent(result *mcp.CallToolResult) string {
	var sb strings.Builder
	for _, c := range result.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			sb.WriteString(tc.Text)
		}
	}
	return sb.String()
}
