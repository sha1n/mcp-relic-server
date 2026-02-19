package gitrepos

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ============================
// Mock-based handler tests
// ============================

func TestNewReadHandler(t *testing.T) {
	handler := NewReadHandler(&mockReadService{})
	if handler == nil {
		t.Fatal("Expected non-nil handler")
	}
}

func TestReadHandler_NotReady(t *testing.T) {
	handler := NewReadHandler(&mockReadService{ready: false})
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
	handler := NewReadHandler(&mockReadService{ready: true})
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
	handler := NewReadHandler(&mockReadService{ready: true})
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

func TestReadHandler_WhitespaceOnlyRepository(t *testing.T) {
	handler := NewReadHandler(&mockReadService{ready: true})
	ctx := context.Background()

	repos := []string{"   ", "\t", "\n"}
	for _, repo := range repos {
		result, _, err := handler.Handle(ctx, &mcp.CallToolRequest{}, ReadArgument{
			Repository: repo,
			Path:       "main.go",
		})
		if err != nil {
			t.Fatalf("Handle returned error for repo %q: %v", repo, err)
		}
		if !result.IsError {
			t.Errorf("Expected error for whitespace-only repository %q", repo)
		}
	}
}

func TestReadHandler_WhitespaceOnlyPath(t *testing.T) {
	handler := NewReadHandler(&mockReadService{ready: true})
	ctx := context.Background()

	paths := []string{"   ", "\t", "\n"}
	for _, path := range paths {
		result, _, err := handler.Handle(ctx, &mcp.CallToolRequest{}, ReadArgument{
			Repository: "github.com/test/repo",
			Path:       path,
		})
		if err != nil {
			t.Fatalf("Handle returned error for path %q: %v", path, err)
		}
		if !result.IsError {
			t.Errorf("Expected error for whitespace-only path %q", path)
		}
	}
}

func TestReadHandler_GetToolDefinition(t *testing.T) {
	handler := NewReadHandler(&mockReadService{})
	tool := handler.GetToolDefinition()

	if tool.Name != "read" {
		t.Errorf("Tool name = %q, want 'read'", tool.Name)
	}
	if tool.Description == "" {
		t.Error("Tool description should not be empty")
	}
	if !strings.Contains(tool.Description, "WHEN TO USE") {
		t.Error("Tool description should contain 'WHEN TO USE' section")
	}
	if !strings.Contains(tool.Description, "HOW IT WORKS") {
		t.Error("Tool description should contain 'HOW IT WORKS' section")
	}
}

// ============================
// Filesystem-based tests (use mockReadService with t.TempDir)
// ============================

func TestReadHandler_ReadValidFile(t *testing.T) {
	repoDir := t.TempDir()
	writeTestFile(t, repoDir, "main.go", "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}")

	handler := NewReadHandler(&mockReadService{ready: true, repoDir: repoDir, maxFileSize: 256 * 1024})
	ctx := context.Background()

	result, _, err := handler.Handle(ctx, &mcp.CallToolRequest{}, ReadArgument{
		Repository: "github.com/test/repo",
		Path:       "main.go",
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if result.IsError {
		t.Errorf("Expected success, got error: %s", ExtractTextContent(result))
	}

	content := ExtractTextContent(result)
	if !strings.Contains(content, "package main") {
		t.Errorf("Expected file content in result, got: %s", content)
	}
}

func TestReadHandler_ReadNestedFile(t *testing.T) {
	repoDir := t.TempDir()
	writeTestFile(t, repoDir, "src/lib/utils.go", "package lib\n\nfunc Helper() {}")

	handler := NewReadHandler(&mockReadService{ready: true, repoDir: repoDir, maxFileSize: 256 * 1024})
	ctx := context.Background()

	result, _, err := handler.Handle(ctx, &mcp.CallToolRequest{}, ReadArgument{
		Repository: "github.com/test/repo",
		Path:       "src/lib/utils.go",
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if result.IsError {
		t.Errorf("Expected success, got error: %s", ExtractTextContent(result))
	}
}

func TestReadHandler_NonExistentRepository(t *testing.T) {
	repoDir := filepath.Join(t.TempDir(), "nonexistent")

	handler := NewReadHandler(&mockReadService{ready: true, repoDir: repoDir, maxFileSize: 256 * 1024})
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

	content := ExtractTextContent(result)
	if !strings.Contains(content, "not found") {
		t.Errorf("Expected 'not found' in error, got: %s", content)
	}
}

func TestReadHandler_NonExistentFile(t *testing.T) {
	repoDir := t.TempDir()
	writeTestFile(t, repoDir, "main.go", "package main")

	handler := NewReadHandler(&mockReadService{ready: true, repoDir: repoDir, maxFileSize: 256 * 1024})
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

	content := ExtractTextContent(result)
	if !strings.Contains(content, "not found") {
		t.Errorf("Expected 'not found' in error, got: %s", content)
	}
}

func TestReadHandler_PathTraversalDotDot(t *testing.T) {
	repoDir := t.TempDir()
	writeTestFile(t, repoDir, "main.go", "package main")

	handler := NewReadHandler(&mockReadService{ready: true, repoDir: repoDir, maxFileSize: 256 * 1024})
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
	repoDir := t.TempDir()
	writeTestFile(t, repoDir, "main.go", "package main")

	handler := NewReadHandler(&mockReadService{ready: true, repoDir: repoDir, maxFileSize: 256 * 1024})
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
	repoDir := t.TempDir()
	writeTestFile(t, repoDir, "src/main.go", "package main")

	handler := NewReadHandler(&mockReadService{ready: true, repoDir: repoDir, maxFileSize: 256 * 1024})
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

	content := ExtractTextContent(result)
	if !strings.Contains(content, "directory") {
		t.Errorf("Expected 'directory' in error, got: %s", content)
	}
}

func TestReadHandler_FileTooLarge(t *testing.T) {
	repoDir := t.TempDir()
	largeContent := strings.Repeat("x", 1024) // 1KB
	writeTestFile(t, repoDir, "large.txt", largeContent)

	handler := NewReadHandler(&mockReadService{ready: true, repoDir: repoDir, maxFileSize: 500}) // Only 500 bytes
	ctx := context.Background()

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

	content := ExtractTextContent(result)
	if !strings.Contains(content, "too large") {
		t.Errorf("Expected 'too large' in error, got: %s", content)
	}
}

func TestReadHandler_BinaryFile(t *testing.T) {
	repoDir := t.TempDir()
	binaryPath := filepath.Join(repoDir, "binary.dat")
	binaryContent := []byte{'B', 'I', 'N', 0x00, 'A', 'R', 'Y'}
	if err := os.WriteFile(binaryPath, binaryContent, 0644); err != nil {
		t.Fatalf("Failed to write binary file: %v", err)
	}

	handler := NewReadHandler(&mockReadService{ready: true, repoDir: repoDir, maxFileSize: 256 * 1024})
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

	content := ExtractTextContent(result)
	if !strings.Contains(content, "binary") {
		t.Errorf("Expected 'binary' in error, got: %s", content)
	}
}

func TestReadHandler_ResultFormat(t *testing.T) {
	repoDir := t.TempDir()
	writeTestFile(t, repoDir, "main.go", "package main\n\nfunc main() {}\n")

	handler := NewReadHandler(&mockReadService{ready: true, repoDir: repoDir, maxFileSize: 256 * 1024})
	ctx := context.Background()

	result, _, err := handler.Handle(ctx, &mcp.CallToolRequest{}, ReadArgument{
		Repository: "github.com/test/repo",
		Path:       "main.go",
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Expected success, got error: %s", ExtractTextContent(result))
	}

	content := ExtractTextContent(result)

	if !strings.Contains(content, "**github.com/test/repo** `main.go`") {
		t.Errorf("Expected header '**github.com/test/repo** `main.go`' in output, got: %s", content)
	}
	if !strings.Contains(content, "```go") {
		t.Errorf("Expected '```go' code fence in output, got: %s", content)
	}
	if !strings.Contains(content, "```\n") {
		t.Errorf("Expected closing code fence in output, got: %s", content)
	}
	if !strings.Contains(content, "package main") {
		t.Errorf("Expected file content in output, got: %s", content)
	}
}

func TestReadHandler_LanguageHint(t *testing.T) {
	repoDir := t.TempDir()
	writeTestFile(t, repoDir, "app.py", "def main():\n    pass\n")
	writeTestFile(t, repoDir, "script.sh", "#!/bin/bash\necho hello\n")
	writeTestFile(t, repoDir, "config.yml", "key: value\n")

	handler := NewReadHandler(&mockReadService{ready: true, repoDir: repoDir, maxFileSize: 256 * 1024})
	ctx := context.Background()

	tests := []struct {
		path     string
		wantLang string
	}{
		{"app.py", "```python"},
		{"script.sh", "```bash"},
		{"config.yml", "```yaml"},
	}

	for _, tt := range tests {
		result, _, err := handler.Handle(ctx, &mcp.CallToolRequest{}, ReadArgument{
			Repository: "github.com/test/repo",
			Path:       tt.path,
		})
		if err != nil {
			t.Fatalf("Handle returned error for %s: %v", tt.path, err)
		}
		if result.IsError {
			t.Fatalf("Expected success for %s, got error: %s", tt.path, ExtractTextContent(result))
		}

		content := ExtractTextContent(result)
		if !strings.Contains(content, tt.wantLang) {
			t.Errorf("For %s, expected %q code fence, got: %s", tt.path, tt.wantLang, content)
		}
	}
}

func TestReadHandler_FileWithNoExtension(t *testing.T) {
	repoDir := t.TempDir()
	writeTestFile(t, repoDir, "Makefile", "all:\n\techo build\n")

	handler := NewReadHandler(&mockReadService{ready: true, repoDir: repoDir, maxFileSize: 256 * 1024})
	ctx := context.Background()

	result, _, err := handler.Handle(ctx, &mcp.CallToolRequest{}, ReadArgument{
		Repository: "github.com/test/repo",
		Path:       "Makefile",
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Expected success, got error: %s", ExtractTextContent(result))
	}

	content := ExtractTextContent(result)
	if !strings.Contains(content, "```") {
		t.Errorf("Expected code fence in output for file without extension, got: %s", content)
	}
	if !strings.Contains(content, "echo build") {
		t.Errorf("Expected file content in output, got: %s", content)
	}
}

// ============================
// Pure unit tests for helpers
// ============================

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

// ============================
// Helpers
// ============================

func writeTestFile(t *testing.T, baseDir, relPath, content string) {
	t.Helper()
	fullPath := filepath.Join(baseDir, relPath)
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}
}
