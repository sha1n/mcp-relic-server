package gitrepos

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// CommandExecutor abstracts command execution for testing.
type CommandExecutor interface {
	// Run executes a command and returns its combined output.
	Run(ctx context.Context, dir string, name string, args ...string) ([]byte, error)
}

// DefaultExecutor executes commands using os/exec.
type DefaultExecutor struct{}

// Run executes a command and returns its combined output.
func (e *DefaultExecutor) Run(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		// Include stderr in error message for debugging
		if stderr.Len() > 0 {
			return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
		}
		return nil, err
	}

	return stdout.Bytes(), nil
}

// GitClient executes git commands.
type GitClient struct {
	executor CommandExecutor
}

// NewGitClient creates a new GitClient with the default command executor.
func NewGitClient() *GitClient {
	return &GitClient{
		executor: &DefaultExecutor{},
	}
}

// NewGitClientWithExecutor creates a GitClient with a custom executor (for testing).
func NewGitClientWithExecutor(executor CommandExecutor) *GitClient {
	return &GitClient{
		executor: executor,
	}
}

// Clone performs a shallow clone of the repository.
// Uses --depth 1 and --single-branch for efficiency.
func (g *GitClient) Clone(ctx context.Context, url, destDir string) error {
	_, err := g.executor.Run(ctx, "", "git", "clone",
		"--depth", "1",
		"--single-branch",
		url,
		destDir,
	)
	if err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}
	return nil
}

// Fetch fetches the latest changes from the remote.
// Uses --depth 1 to maintain shallow clone.
func (g *GitClient) Fetch(ctx context.Context, repoDir string) error {
	_, err := g.executor.Run(ctx, repoDir, "git", "fetch", "--depth", "1")
	if err != nil {
		return fmt.Errorf("git fetch failed: %w", err)
	}
	return nil
}

// Reset performs a hard reset to origin/HEAD.
// This updates the working directory to match the remote.
func (g *GitClient) Reset(ctx context.Context, repoDir string) error {
	_, err := g.executor.Run(ctx, repoDir, "git", "reset", "--hard", "origin/HEAD")
	if err != nil {
		return fmt.Errorf("git reset failed: %w", err)
	}
	return nil
}

// GetHeadCommit returns the current HEAD commit SHA.
func (g *GitClient) GetHeadCommit(ctx context.Context, repoDir string) (string, error) {
	output, err := g.executor.Run(ctx, repoDir, "git", "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("git rev-parse failed: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// GetChangedFiles returns the list of files changed between two commits.
// Returns file paths relative to the repository root.
func (g *GitClient) GetChangedFiles(ctx context.Context, repoDir, fromCommit, toCommit string) ([]string, error) {
	output, err := g.executor.Run(ctx, repoDir, "git", "diff",
		"--name-only",
		fromCommit+".."+toCommit,
	)
	if err != nil {
		return nil, fmt.Errorf("git diff failed: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	// Filter empty lines
	var files []string
	for _, line := range lines {
		if line != "" {
			files = append(files, line)
		}
	}

	return files, nil
}

// GetDefaultBranch returns the default branch name (e.g., "main" or "master").
func (g *GitClient) GetDefaultBranch(ctx context.Context, repoDir string) (string, error) {
	// Try to get the default branch from remote HEAD
	output, err := g.executor.Run(ctx, repoDir, "git", "symbolic-ref", "refs/remotes/origin/HEAD")
	if err == nil {
		// Output is like "refs/remotes/origin/main"
		ref := strings.TrimSpace(string(output))
		parts := strings.Split(ref, "/")
		if len(parts) > 0 {
			return parts[len(parts)-1], nil
		}
	}

	// Fallback: check if main exists, then master
	_, err = g.executor.Run(ctx, repoDir, "git", "rev-parse", "--verify", "origin/main")
	if err == nil {
		return "main", nil
	}

	_, err = g.executor.Run(ctx, repoDir, "git", "rev-parse", "--verify", "origin/master")
	if err == nil {
		return "master", nil
	}

	return "", fmt.Errorf("could not determine default branch")
}

// IsGitRepository checks if the given directory is a git repository.
func (g *GitClient) IsGitRepository(ctx context.Context, dir string) bool {
	_, err := g.executor.Run(ctx, dir, "git", "rev-parse", "--git-dir")
	return err == nil
}

// Clean removes untracked files and directories.
func (g *GitClient) Clean(ctx context.Context, repoDir string) error {
	_, err := g.executor.Run(ctx, repoDir, "git", "clean", "-fdx")
	if err != nil {
		return fmt.Errorf("git clean failed: %w", err)
	}
	return nil
}
