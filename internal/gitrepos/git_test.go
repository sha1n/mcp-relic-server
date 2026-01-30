package gitrepos

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// MockExecutor records commands and returns configured responses.
type MockExecutor struct {
	commands []MockCommand
	calls    []ExecutorCall
}

type MockCommand struct {
	NamePrefix string
	Output     []byte
	Err        error
}

type ExecutorCall struct {
	Dir  string
	Name string
	Args []string
}

func NewMockExecutor() *MockExecutor {
	return &MockExecutor{
		commands: make([]MockCommand, 0),
		calls:    make([]ExecutorCall, 0),
	}
}

func (m *MockExecutor) AddResponse(namePrefix string, output []byte, err error) {
	m.commands = append(m.commands, MockCommand{
		NamePrefix: namePrefix,
		Output:     output,
		Err:        err,
	})
}

func (m *MockExecutor) Run(_ context.Context, dir string, name string, args ...string) ([]byte, error) {
	call := ExecutorCall{Dir: dir, Name: name, Args: args}
	m.calls = append(m.calls, call)

	// Build full command string for matching
	fullCmd := name + " " + strings.Join(args, " ")

	// Find matching response
	for i, cmd := range m.commands {
		if strings.HasPrefix(fullCmd, cmd.NamePrefix) {
			// Remove used response
			m.commands = append(m.commands[:i], m.commands[i+1:]...)
			return cmd.Output, cmd.Err
		}
	}

	return nil, errors.New("no mock response configured for: " + fullCmd)
}

func (m *MockExecutor) GetCalls() []ExecutorCall {
	return m.calls
}

// MustGetLastCall returns the last recorded call, panics if no calls.
// Should only be used in tests after verifying a call was made.
func (m *MockExecutor) MustGetLastCall(t *testing.T) ExecutorCall {
	t.Helper()
	if len(m.calls) == 0 {
		t.Fatal("Expected at least one command call")
	}
	return m.calls[len(m.calls)-1]
}

func TestNewGitClient(t *testing.T) {
	client := NewGitClient()
	if client.executor == nil {
		t.Error("Expected executor to be set")
	}
}

func TestNewGitClientWithExecutor(t *testing.T) {
	mock := NewMockExecutor()
	client := NewGitClientWithExecutor(mock)

	if client.executor != mock {
		t.Error("Expected custom executor to be used")
	}
}

func TestGitClient_Clone(t *testing.T) {
	mock := NewMockExecutor()
	mock.AddResponse("git clone", []byte(""), nil)

	client := NewGitClientWithExecutor(mock)
	ctx := context.Background()

	err := client.Clone(ctx, "git@github.com:org/repo.git", "/tmp/dest")
	if err != nil {
		t.Fatalf("Clone failed: %v", err)
	}

	call := mock.MustGetLastCall(t)
	if call.Name != "git" {
		t.Errorf("Expected git command, got %s", call.Name)
	}

	expectedArgs := []string{"clone", "--depth", "1", "--single-branch", "git@github.com:org/repo.git", "/tmp/dest"}
	if len(call.Args) != len(expectedArgs) {
		t.Fatalf("Expected %d args, got %d: %v", len(expectedArgs), len(call.Args), call.Args)
	}

	for i, arg := range expectedArgs {
		if call.Args[i] != arg {
			t.Errorf("Arg[%d] = %q, want %q", i, call.Args[i], arg)
		}
	}
}

func TestGitClient_Clone_Error(t *testing.T) {
	mock := NewMockExecutor()
	mock.AddResponse("git clone", nil, errors.New("authentication failed"))

	client := NewGitClientWithExecutor(mock)
	ctx := context.Background()

	err := client.Clone(ctx, "git@github.com:org/repo.git", "/tmp/dest")
	if err == nil {
		t.Fatal("Expected error")
	}
	if !strings.Contains(err.Error(), "git clone failed") {
		t.Errorf("Expected 'git clone failed' in error, got: %v", err)
	}
}

func TestGitClient_Fetch(t *testing.T) {
	mock := NewMockExecutor()
	mock.AddResponse("git fetch", []byte(""), nil)

	client := NewGitClientWithExecutor(mock)
	ctx := context.Background()

	err := client.Fetch(ctx, "/tmp/repo")
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	call := mock.MustGetLastCall(t)
	if call.Dir != "/tmp/repo" {
		t.Errorf("Expected dir '/tmp/repo', got %q", call.Dir)
	}

	expectedArgs := []string{"fetch", "--depth", "1"}
	if len(call.Args) != len(expectedArgs) {
		t.Fatalf("Expected %d args, got %d", len(expectedArgs), len(call.Args))
	}
}

func TestGitClient_Fetch_Error(t *testing.T) {
	mock := NewMockExecutor()
	mock.AddResponse("git fetch", nil, errors.New("network error"))

	client := NewGitClientWithExecutor(mock)
	ctx := context.Background()

	err := client.Fetch(ctx, "/tmp/repo")
	if err == nil {
		t.Fatal("Expected error")
	}
	if !strings.Contains(err.Error(), "git fetch failed") {
		t.Errorf("Expected 'git fetch failed' in error, got: %v", err)
	}
}

func TestGitClient_Reset(t *testing.T) {
	mock := NewMockExecutor()
	mock.AddResponse("git reset", []byte(""), nil)

	client := NewGitClientWithExecutor(mock)
	ctx := context.Background()

	err := client.Reset(ctx, "/tmp/repo")
	if err != nil {
		t.Fatalf("Reset failed: %v", err)
	}

	call := mock.MustGetLastCall(t)
	expectedArgs := []string{"reset", "--hard", "origin/HEAD"}
	if len(call.Args) != len(expectedArgs) {
		t.Fatalf("Expected %d args, got %d", len(expectedArgs), len(call.Args))
	}
	for i, arg := range expectedArgs {
		if call.Args[i] != arg {
			t.Errorf("Arg[%d] = %q, want %q", i, call.Args[i], arg)
		}
	}
}

func TestGitClient_Reset_Error(t *testing.T) {
	mock := NewMockExecutor()
	mock.AddResponse("git reset", nil, errors.New("merge conflict"))

	client := NewGitClientWithExecutor(mock)
	ctx := context.Background()

	err := client.Reset(ctx, "/tmp/repo")
	if err == nil {
		t.Fatal("Expected error")
	}
	if !strings.Contains(err.Error(), "git reset failed") {
		t.Errorf("Expected 'git reset failed' in error, got: %v", err)
	}
}

func TestGitClient_GetHeadCommit(t *testing.T) {
	mock := NewMockExecutor()
	mock.AddResponse("git rev-parse HEAD", []byte("abc123def456\n"), nil)

	client := NewGitClientWithExecutor(mock)
	ctx := context.Background()

	commit, err := client.GetHeadCommit(ctx, "/tmp/repo")
	if err != nil {
		t.Fatalf("GetHeadCommit failed: %v", err)
	}

	if commit != "abc123def456" {
		t.Errorf("Expected commit 'abc123def456', got %q", commit)
	}
}

func TestGitClient_GetHeadCommit_TrimsWhitespace(t *testing.T) {
	mock := NewMockExecutor()
	mock.AddResponse("git rev-parse HEAD", []byte("  abc123def456  \n\n"), nil)

	client := NewGitClientWithExecutor(mock)
	ctx := context.Background()

	commit, err := client.GetHeadCommit(ctx, "/tmp/repo")
	if err != nil {
		t.Fatalf("GetHeadCommit failed: %v", err)
	}

	if commit != "abc123def456" {
		t.Errorf("Expected trimmed commit, got %q", commit)
	}
}

func TestGitClient_GetHeadCommit_Error(t *testing.T) {
	mock := NewMockExecutor()
	mock.AddResponse("git rev-parse", nil, errors.New("not a git repository"))

	client := NewGitClientWithExecutor(mock)
	ctx := context.Background()

	_, err := client.GetHeadCommit(ctx, "/tmp/repo")
	if err == nil {
		t.Fatal("Expected error")
	}
	if !strings.Contains(err.Error(), "git rev-parse failed") {
		t.Errorf("Expected 'git rev-parse failed' in error, got: %v", err)
	}
}

func TestGitClient_GetChangedFiles(t *testing.T) {
	mock := NewMockExecutor()
	mock.AddResponse("git diff", []byte("src/main.go\nsrc/utils.go\nREADME.md\n"), nil)

	client := NewGitClientWithExecutor(mock)
	ctx := context.Background()

	files, err := client.GetChangedFiles(ctx, "/tmp/repo", "abc123", "def456")
	if err != nil {
		t.Fatalf("GetChangedFiles failed: %v", err)
	}

	expected := []string{"src/main.go", "src/utils.go", "README.md"}
	if len(files) != len(expected) {
		t.Fatalf("Expected %d files, got %d: %v", len(expected), len(files), files)
	}

	for i, file := range expected {
		if files[i] != file {
			t.Errorf("files[%d] = %q, want %q", i, files[i], file)
		}
	}

	call := mock.MustGetLastCall(t)
	if call.Args[0] != "diff" || call.Args[1] != "--name-only" || call.Args[2] != "abc123..def456" {
		t.Errorf("Unexpected args: %v", call.Args)
	}
}

func TestGitClient_GetChangedFiles_EmptyOutput(t *testing.T) {
	mock := NewMockExecutor()
	mock.AddResponse("git diff", []byte(""), nil)

	client := NewGitClientWithExecutor(mock)
	ctx := context.Background()

	files, err := client.GetChangedFiles(ctx, "/tmp/repo", "abc123", "def456")
	if err != nil {
		t.Fatalf("GetChangedFiles failed: %v", err)
	}

	if len(files) != 0 {
		t.Errorf("Expected empty file list, got %v", files)
	}
}

func TestGitClient_GetChangedFiles_FilterEmptyLines(t *testing.T) {
	mock := NewMockExecutor()
	mock.AddResponse("git diff", []byte("file1.go\n\nfile2.go\n\n\n"), nil)

	client := NewGitClientWithExecutor(mock)
	ctx := context.Background()

	files, err := client.GetChangedFiles(ctx, "/tmp/repo", "abc", "def")
	if err != nil {
		t.Fatalf("GetChangedFiles failed: %v", err)
	}

	if len(files) != 2 {
		t.Errorf("Expected 2 files (empty lines filtered), got %d: %v", len(files), files)
	}
}

func TestGitClient_GetChangedFiles_Error(t *testing.T) {
	mock := NewMockExecutor()
	mock.AddResponse("git diff", nil, errors.New("bad revision"))

	client := NewGitClientWithExecutor(mock)
	ctx := context.Background()

	_, err := client.GetChangedFiles(ctx, "/tmp/repo", "invalid", "commits")
	if err == nil {
		t.Fatal("Expected error")
	}
	if !strings.Contains(err.Error(), "git diff failed") {
		t.Errorf("Expected 'git diff failed' in error, got: %v", err)
	}
}

func TestGitClient_GetDefaultBranch_Main(t *testing.T) {
	mock := NewMockExecutor()
	mock.AddResponse("git symbolic-ref", []byte("refs/remotes/origin/main\n"), nil)

	client := NewGitClientWithExecutor(mock)
	ctx := context.Background()

	branch, err := client.GetDefaultBranch(ctx, "/tmp/repo")
	if err != nil {
		t.Fatalf("GetDefaultBranch failed: %v", err)
	}

	if branch != "main" {
		t.Errorf("Expected 'main', got %q", branch)
	}
}

func TestGitClient_GetDefaultBranch_Master(t *testing.T) {
	mock := NewMockExecutor()
	mock.AddResponse("git symbolic-ref", []byte("refs/remotes/origin/master\n"), nil)

	client := NewGitClientWithExecutor(mock)
	ctx := context.Background()

	branch, err := client.GetDefaultBranch(ctx, "/tmp/repo")
	if err != nil {
		t.Fatalf("GetDefaultBranch failed: %v", err)
	}

	if branch != "master" {
		t.Errorf("Expected 'master', got %q", branch)
	}
}

func TestGitClient_GetDefaultBranch_FallbackToMain(t *testing.T) {
	mock := NewMockExecutor()
	// symbolic-ref fails
	mock.AddResponse("git symbolic-ref", nil, errors.New("not found"))
	// rev-parse origin/main succeeds
	mock.AddResponse("git rev-parse --verify origin/main", []byte("abc123\n"), nil)

	client := NewGitClientWithExecutor(mock)
	ctx := context.Background()

	branch, err := client.GetDefaultBranch(ctx, "/tmp/repo")
	if err != nil {
		t.Fatalf("GetDefaultBranch failed: %v", err)
	}

	if branch != "main" {
		t.Errorf("Expected 'main' from fallback, got %q", branch)
	}
}

func TestGitClient_GetDefaultBranch_FallbackToMaster(t *testing.T) {
	mock := NewMockExecutor()
	// symbolic-ref fails
	mock.AddResponse("git symbolic-ref", nil, errors.New("not found"))
	// rev-parse origin/main fails
	mock.AddResponse("git rev-parse --verify origin/main", nil, errors.New("not found"))
	// rev-parse origin/master succeeds
	mock.AddResponse("git rev-parse --verify origin/master", []byte("def456\n"), nil)

	client := NewGitClientWithExecutor(mock)
	ctx := context.Background()

	branch, err := client.GetDefaultBranch(ctx, "/tmp/repo")
	if err != nil {
		t.Fatalf("GetDefaultBranch failed: %v", err)
	}

	if branch != "master" {
		t.Errorf("Expected 'master' from fallback, got %q", branch)
	}
}

func TestGitClient_GetDefaultBranch_Error(t *testing.T) {
	mock := NewMockExecutor()
	mock.AddResponse("git symbolic-ref", nil, errors.New("not found"))
	mock.AddResponse("git rev-parse --verify origin/main", nil, errors.New("not found"))
	mock.AddResponse("git rev-parse --verify origin/master", nil, errors.New("not found"))

	client := NewGitClientWithExecutor(mock)
	ctx := context.Background()

	_, err := client.GetDefaultBranch(ctx, "/tmp/repo")
	if err == nil {
		t.Fatal("Expected error")
	}
	if !strings.Contains(err.Error(), "could not determine default branch") {
		t.Errorf("Expected specific error message, got: %v", err)
	}
}

func TestGitClient_IsGitRepository_True(t *testing.T) {
	mock := NewMockExecutor()
	mock.AddResponse("git rev-parse --git-dir", []byte(".git\n"), nil)

	client := NewGitClientWithExecutor(mock)
	ctx := context.Background()

	result := client.IsGitRepository(ctx, "/tmp/repo")
	if !result {
		t.Error("Expected true for valid repository")
	}
}

func TestGitClient_IsGitRepository_False(t *testing.T) {
	mock := NewMockExecutor()
	mock.AddResponse("git rev-parse --git-dir", nil, errors.New("not a git repository"))

	client := NewGitClientWithExecutor(mock)
	ctx := context.Background()

	result := client.IsGitRepository(ctx, "/tmp/not-a-repo")
	if result {
		t.Error("Expected false for non-repository")
	}
}

func TestGitClient_Clean(t *testing.T) {
	mock := NewMockExecutor()
	mock.AddResponse("git clean", []byte(""), nil)

	client := NewGitClientWithExecutor(mock)
	ctx := context.Background()

	err := client.Clean(ctx, "/tmp/repo")
	if err != nil {
		t.Fatalf("Clean failed: %v", err)
	}

	call := mock.MustGetLastCall(t)
	expectedArgs := []string{"clean", "-fdx"}
	if len(call.Args) != len(expectedArgs) {
		t.Fatalf("Expected %d args, got %d", len(expectedArgs), len(call.Args))
	}
	for i, arg := range expectedArgs {
		if call.Args[i] != arg {
			t.Errorf("Arg[%d] = %q, want %q", i, call.Args[i], arg)
		}
	}
}

func TestGitClient_Clean_Error(t *testing.T) {
	mock := NewMockExecutor()
	mock.AddResponse("git clean", nil, errors.New("permission denied"))

	client := NewGitClientWithExecutor(mock)
	ctx := context.Background()

	err := client.Clean(ctx, "/tmp/repo")
	if err == nil {
		t.Fatal("Expected error")
	}
	if !strings.Contains(err.Error(), "git clean failed") {
		t.Errorf("Expected 'git clean failed' in error, got: %v", err)
	}
}

func TestDefaultExecutor_Run(t *testing.T) {
	executor := &DefaultExecutor{}
	ctx := context.Background()

	// Test with a simple command that should work everywhere
	output, err := executor.Run(ctx, "", "echo", "hello")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if !strings.Contains(string(output), "hello") {
		t.Errorf("Expected 'hello' in output, got %q", string(output))
	}
}

func TestDefaultExecutor_Run_WithDir(t *testing.T) {
	executor := &DefaultExecutor{}
	ctx := context.Background()

	// Run pwd in temp directory
	tmpDir := t.TempDir()
	output, err := executor.Run(ctx, tmpDir, "pwd")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if !strings.Contains(string(output), tmpDir) {
		t.Errorf("Expected directory in output, got %q", string(output))
	}
}

func TestDefaultExecutor_Run_Error(t *testing.T) {
	executor := &DefaultExecutor{}
	ctx := context.Background()

	// Run a command that doesn't exist
	_, err := executor.Run(ctx, "", "nonexistent-command-xyz")
	if err == nil {
		t.Error("Expected error for nonexistent command")
	}
}

func TestDefaultExecutor_Run_ContextCancellation(t *testing.T) {
	executor := &DefaultExecutor{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Run a command with cancelled context
	_, err := executor.Run(ctx, "", "sleep", "10")
	if err == nil {
		t.Error("Expected error for cancelled context")
	}
}
