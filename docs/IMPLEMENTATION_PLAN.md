# RELIC MCP Server - Implementation Plan

This document outlines the implementation plan for the Git repository indexing and code search functionality as specified in [SPEC.md](./SPEC.md).

## Overview

The implementation is divided into **13 discrete tasks**, each containing code with corresponding unit and integration tests. Every task will be committed separately after passing `make lint test`.

**Coverage Target**: 100% test coverage for new code.

---

## Task Dependencies

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              DEPENDENCY GRAPH                               │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  Task 1: GitReposSettings ──────────────────────┐                           │
│                                                 │                           │
│  Task 2: Domain Models ─────────────────────────┤                           │
│                                                 │                           │
│  Task 3: URL Utilities ─────────────────────────┼──► Task 5: Git Operations │
│                                                 │              │            │
│  Task 4: File Lock ─────────────────────────────┤              │            │
│                                                 │              ▼            │
│                                         Task 6: Manifest ──────┤            │
│                                                                │            │
│  Task 7: File Filtering ────────────────────────┐              │            │
│                                                 │              │            │
│                                                 ▼              ▼            │
│                                         Task 8: Indexer ───────┤            │
│                                                                │            │
│                                                                ▼            │
│                                         Task 9: Service ───────┤            │
│                                                                │            │
│                                                                ▼            │
│                                     Task 10: Search Tool ──────┤            │
│                                                                │            │
│                                     Task 11: Read Tool ────────┤            │
│                                                                │            │
│                                                                ▼            │
│                                     Task 12: MCP Registration ─┤            │
│                                                                │            │
│                                                                ▼            │
│                                     Task 13: Integration Tests             │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Task 1: Add GitReposSettings to Configuration

**Package**: `internal/config`

**Files to Create/Modify**:
- `internal/config/settings.go` - Add `GitReposSettings` struct
- `internal/config/settings_test.go` - Add tests for new settings

**Implementation Details**:

```go
// GitReposSettings contains configuration for git repository indexing
type GitReposSettings struct {
    Enabled      bool          `mapstructure:"enabled"`
    URLs         []string      `mapstructure:"urls"`
    BaseDir      string        `mapstructure:"base_dir"`
    SyncInterval time.Duration `mapstructure:"sync_interval"`
    SyncTimeout  time.Duration `mapstructure:"sync_timeout"`
    MaxFileSize  int64         `mapstructure:"max_file_size"`
    MaxResults   int           `mapstructure:"max_results"`
}
```

**Configuration Mapping**:
| Environment Variable | CLI Flag | Default |
|---------------------|----------|---------|
| `RELIC_MCP_GIT_REPOS_ENABLED` | `--git-repos-enabled` | `false` |
| `RELIC_MCP_GIT_REPOS_URLS` | `--git-repos-urls` | `""` |
| `RELIC_MCP_GIT_REPOS_BASE_DIR` | `--git-repos-base-dir` | `~/.relic-mcp` |
| `RELIC_MCP_GIT_REPOS_SYNC_INTERVAL` | `--git-repos-sync-interval` | `15m` |
| `RELIC_MCP_GIT_REPOS_SYNC_TIMEOUT` | `--git-repos-sync-timeout` | `60s` |
| `RELIC_MCP_GIT_REPOS_MAX_FILE_SIZE` | `--git-repos-max-file-size` | `262144` (256KB) |
| `RELIC_MCP_GIT_REPOS_MAX_RESULTS` | `--git-repos-max-results` | `20` |

**Unit Tests**:
1. Test default values are correctly set
2. Test environment variable binding for each setting
3. Test CLI flag binding for each setting
4. Test URL parsing from comma-separated string
5. Test home directory expansion in `BaseDir`
6. Test validation:
   - `Enabled=true` requires at least one URL
   - `SyncInterval` must be positive
   - `SyncTimeout` must be positive
   - `MaxFileSize` must be positive
   - `MaxResults` must be positive

**Commit Message**: `feat(config): add GitReposSettings for git repository indexing`

---

## Task 2: Create Domain Models

**Package**: `internal/domain`

**Files to Create**:
- `internal/domain/code.go` - CodeDocument struct and constants
- `internal/domain/code_test.go` - Tests

**Implementation Details**:

```go
// CodeDocument represents an indexed source file
type CodeDocument struct {
    ID         string `json:"id"`          // "github.com_org_repo/path/to/file.go"
    Repository string `json:"repository"`  // "github.com/org/repo"
    FilePath   string `json:"file_path"`   // "src/main/java/App.java"
    Extension  string `json:"extension"`   // "java"
    Content    string `json:"content"`
}

// Field constants for Bleve indexing
const (
    CodeFieldID         = "id"
    CodeFieldRepository = "repository"
    CodeFieldFilePath   = "file_path"
    CodeFieldExtension  = "extension"
    CodeFieldContent    = "content"
)
```

**Unit Tests**:
1. Test CodeDocument JSON marshaling/unmarshaling
2. Test field constants exist and have expected values

**Commit Message**: `feat(domain): add CodeDocument model for indexed files`

---

## Task 3: URL Utilities

**Package**: `internal/gitrepos`

**Files to Create**:
- `internal/gitrepos/url.go` - URL parsing and repo ID generation
- `internal/gitrepos/url_test.go` - Tests

**Implementation Details**:

```go
// URLToRepoID converts SSH URL to filesystem-safe repo ID
// "git@github.com:org/repo.git" -> "github.com_org_repo"
func URLToRepoID(url string) string

// RepoIDToDisplay converts repo ID back to display format
// "github.com_org_repo" -> "github.com/org/repo"
func RepoIDToDisplay(repoID string) string

// DisplayToRepoID converts display format to repo ID
// "github.com/org/repo" -> "github.com_org_repo"
func DisplayToRepoID(display string) string

// ParseSSHURL parses SSH URL into components
// Returns: host, org, repo, error
func ParseSSHURL(url string) (host, org, repo string, err error)
```

**Unit Tests**:
1. Test `URLToRepoID` with various SSH URL formats:
   - `git@github.com:org/repo.git`
   - `git@github.com:org/repo` (no .git suffix)
   - `git@gitlab.com:group/subgroup/repo.git`
   - `ssh://git@github.com/org/repo.git`
2. Test `RepoIDToDisplay` conversion
3. Test `DisplayToRepoID` conversion
4. Test `ParseSSHURL` with valid and invalid URLs
5. Test round-trip: URL → RepoID → Display → RepoID

**Commit Message**: `feat(gitrepos): add URL parsing and repo ID utilities`

---

## Task 4: File Lock Implementation

**Package**: `internal/gitrepos`

**Files to Create**:
- `internal/gitrepos/filelock.go` - File locking using flock(2)
- `internal/gitrepos/filelock_test.go` - Tests

**Implementation Details**:

```go
// FileLock provides exclusive file locking using flock(2)
type FileLock struct {
    path string
    file *os.File
}

// NewFileLock creates a new file lock at the given path
func NewFileLock(path string) *FileLock

// TryLock attempts to acquire exclusive lock without blocking
// Returns true if lock acquired, false if would block
func (l *FileLock) TryLock() (bool, error)

// Lock acquires exclusive lock, blocking until available or timeout
func (l *FileLock) Lock(timeout time.Duration) error

// Unlock releases the lock
func (l *FileLock) Unlock() error
```

**Unit Tests**:
1. Test `TryLock` succeeds when lock is available
2. Test `TryLock` returns false when lock is held by another process
3. Test `Lock` with timeout succeeds when lock becomes available
4. Test `Lock` times out when lock is held
5. Test `Unlock` releases lock
6. Test lock file is created in correct location
7. Test concurrent lock acquisition (using goroutines)

**Commit Message**: `feat(gitrepos): add file locking with flock(2)`

---

## Task 5: Git Operations

**Package**: `internal/gitrepos`

**Files to Create**:
- `internal/gitrepos/git.go` - Git command execution
- `internal/gitrepos/git_test.go` - Tests

**Implementation Details**:

```go
// GitClient executes git commands
type GitClient struct {
    // Command executor (interface for testing)
    executor CommandExecutor
}

// CommandExecutor abstracts command execution for testing
type CommandExecutor interface {
    Run(ctx context.Context, dir string, name string, args ...string) ([]byte, error)
}

// Clone performs shallow clone of repository
func (g *GitClient) Clone(ctx context.Context, url, destDir string) error

// Fetch fetches latest changes
func (g *GitClient) Fetch(ctx context.Context, repoDir string) error

// Reset performs hard reset to origin/HEAD
func (g *GitClient) Reset(ctx context.Context, repoDir string) error

// GetHeadCommit returns current HEAD commit SHA
func (g *GitClient) GetHeadCommit(ctx context.Context, repoDir string) (string, error)

// GetChangedFiles returns files changed between two commits
func (g *GitClient) GetChangedFiles(ctx context.Context, repoDir, fromCommit, toCommit string) ([]string, error)
```

**Unit Tests** (using mock CommandExecutor):
1. Test `Clone` executes correct git command with `--depth 1 --single-branch`
2. Test `Clone` handles errors correctly
3. Test `Fetch` executes `git fetch --depth 1`
4. Test `Reset` executes `git reset --hard origin/HEAD`
5. Test `GetHeadCommit` parses commit SHA correctly
6. Test `GetChangedFiles` parses diff output correctly
7. Test context cancellation is respected

**Integration Tests** (using real git):
1. Test cloning a small test repository
2. Test fetch and reset workflow

**Commit Message**: `feat(gitrepos): add git operations (clone, fetch, reset, diff)`

---

## Task 6: Manifest Management

**Package**: `internal/gitrepos`

**Files to Create**:
- `internal/gitrepos/manifest.go` - Manifest read/write operations
- `internal/gitrepos/manifest_test.go` - Tests

**Implementation Details**:

```go
// Manifest stores repository sync state
type Manifest struct {
    Version  int                  `json:"version"`
    LastSync time.Time            `json:"last_sync"`
    Repos    map[string]RepoState `json:"repos"`
}

// RepoState stores state for a single repository
type RepoState struct {
    URL         string    `json:"url"`
    ClonedAt    time.Time `json:"cloned_at"`
    LastPull    time.Time `json:"last_pull"`
    LastCommit  string    `json:"last_commit"`
    LastIndexed string    `json:"last_indexed"`
    FileCount   int       `json:"file_count"`
    Error       string    `json:"error,omitempty"`
}

// LoadManifest reads manifest from disk or creates new one
func LoadManifest(path string) (*Manifest, error)

// Save writes manifest to disk atomically
func (m *Manifest) Save(path string) error

// GetRepoState returns state for a repo, creating if not exists
func (m *Manifest) GetRepoState(repoID string) *RepoState

// UpdateRepoState updates state for a repo
func (m *Manifest) UpdateRepoState(repoID string, state RepoState)

// RemoveStaleRepos removes repos not in the given URL list
func (m *Manifest) RemoveStaleRepos(urls []string)
```

**Unit Tests**:
1. Test `LoadManifest` creates new manifest when file doesn't exist
2. Test `LoadManifest` reads existing manifest correctly
3. Test `Save` writes JSON atomically (write to temp, rename)
4. Test `GetRepoState` creates new state for unknown repo
5. Test `GetRepoState` returns existing state
6. Test `UpdateRepoState` updates correctly
7. Test `RemoveStaleRepos` removes repos not in URL list
8. Test JSON round-trip preserves all fields

**Commit Message**: `feat(gitrepos): add manifest management for sync state`

---

## Task 7: File Filtering

**Package**: `internal/gitrepos`

**Files to Create**:
- `internal/gitrepos/filter.go` - File filtering logic
- `internal/gitrepos/filter_test.go` - Tests

**Implementation Details**:

```go
// DefaultExcludePatterns contains patterns to exclude from indexing
var DefaultExcludePatterns = []string{
    // Dependencies
    "node_modules/**", "vendor/**", "venv/**", ".venv/**",
    "target/**", "build/**", "dist/**", "out/**",
    // Generated
    "*.min.js", "*.min.css", "*.map", "*.pb.go",
    "package-lock.json", "yarn.lock", "pnpm-lock.yaml",
    "go.sum", "poetry.lock", "Cargo.lock",
    // Binary/Media
    "*.png", "*.jpg", "*.jpeg", "*.gif", "*.ico", "*.svg",
    "*.woff", "*.woff2", "*.ttf", "*.eot",
    "*.zip", "*.tar", "*.gz", "*.rar",
    "*.exe", "*.dll", "*.so", "*.dylib",
    "*.pdf", "*.doc", "*.docx",
}

// FileFilter determines if files should be indexed
type FileFilter struct {
    patterns    []string
    maxFileSize int64
}

// NewFileFilter creates a filter with default patterns
func NewFileFilter(maxFileSize int64) *FileFilter

// ShouldExclude returns true if the path matches exclusion patterns
func (f *FileFilter) ShouldExclude(relPath string) bool

// IsBinary checks if content appears to be binary
// Returns true if first 512 bytes contain null bytes
func IsBinary(content []byte) bool
```

**Unit Tests**:
1. Test `ShouldExclude` with node_modules paths
2. Test `ShouldExclude` with vendor paths
3. Test `ShouldExclude` with binary extensions
4. Test `ShouldExclude` with lock files
5. Test `ShouldExclude` allows normal source files
6. Test `ShouldExclude` with nested paths (e.g., `src/vendor/file.go` should match)
7. Test `IsBinary` returns true for content with null bytes
8. Test `IsBinary` returns false for text content
9. Test `IsBinary` handles empty content
10. Test `IsBinary` handles content shorter than 512 bytes

**Commit Message**: `feat(gitrepos): add file filtering for indexing`

---

## Task 8: Bleve Indexer

**Package**: `internal/gitrepos`

**Files to Create**:
- `internal/gitrepos/indexer.go` - Bleve indexing logic
- `internal/gitrepos/indexer_test.go` - Tests

**Dependencies to Add**:
- `github.com/blevesearch/bleve/v2`

**Implementation Details**:

```go
// Indexer manages Bleve indexes for repositories
type Indexer struct {
    baseDir     string
    filter      *FileFilter
    maxFileSize int64
}

// NewIndexer creates a new indexer
func NewIndexer(baseDir string, filter *FileFilter, maxFileSize int64) *Indexer

// OpenForWrite opens or creates index for writing
func (i *Indexer) OpenForWrite(repoID string) (bleve.Index, error)

// OpenForRead opens index for reading (read-only mode)
func (i *Indexer) OpenForRead(repoID string) (bleve.Index, error)

// CreateAlias creates an alias combining multiple indexes
func (i *Indexer) CreateAlias(repoIDs []string) (bleve.IndexAlias, error)

// FullIndex performs full indexing of a repository
func (i *Indexer) FullIndex(repoID, repoDir string) (fileCount int, err error)

// IncrementalIndex updates index for changed files only
func (i *Indexer) IncrementalIndex(repoID, repoDir string, changedFiles []string) error

// DeleteIndex removes an index from disk
func (i *Indexer) DeleteIndex(repoID string) error
```

**Bleve Index Mapping**:
```go
func createIndexMapping() mapping.IndexMapping {
    docMapping := bleve.NewDocumentMapping()

    // Content field - analyzed for full-text search
    contentField := bleve.NewTextFieldMapping()
    contentField.Analyzer = "standard"
    docMapping.AddFieldMappingsAt(CodeFieldContent, contentField)

    // Repository - keyword (not analyzed)
    repoField := bleve.NewTextFieldMapping()
    repoField.Analyzer = "keyword"
    docMapping.AddFieldMappingsAt(CodeFieldRepository, repoField)

    // Extension - keyword, lowercase
    extField := bleve.NewTextFieldMapping()
    extField.Analyzer = "keyword"
    docMapping.AddFieldMappingsAt(CodeFieldExtension, extField)

    // FilePath - keyword
    pathField := bleve.NewTextFieldMapping()
    pathField.Analyzer = "keyword"
    docMapping.AddFieldMappingsAt(CodeFieldFilePath, pathField)

    indexMapping := bleve.NewIndexMapping()
    indexMapping.DefaultMapping = docMapping
    return indexMapping
}
```

**Unit Tests**:
1. Test `OpenForWrite` creates new index
2. Test `OpenForWrite` opens existing index
3. Test `OpenForRead` opens index in read-only mode
4. Test `CreateAlias` combines multiple indexes
5. Test `FullIndex` indexes all files in directory
6. Test `FullIndex` respects file filter
7. Test `FullIndex` respects max file size
8. Test `FullIndex` skips binary files
9. Test `FullIndex` skips .git directory
10. Test `FullIndex` batches documents correctly
11. Test `IncrementalIndex` updates changed files
12. Test `IncrementalIndex` removes deleted files
13. Test `DeleteIndex` removes index from disk

**Commit Message**: `feat(gitrepos): add Bleve indexer for full-text search`

---

## Task 9: Git Repos Service

**Package**: `internal/gitrepos`

**Files to Create**:
- `internal/gitrepos/service.go` - Main service coordinating all operations
- `internal/gitrepos/service_test.go` - Tests

**Implementation Details**:

```go
// Service coordinates git operations, indexing, and search
type Service struct {
    settings  *config.GitReposSettings
    git       *GitClient
    indexer   *Indexer
    filter    *FileFilter
    manifest  *Manifest
    lock      *FileLock
    alias     bleve.IndexAlias // Combined index for search
    ready     bool             // True when indexes are ready
    mu        sync.RWMutex     // Protects ready and alias
}

// NewService creates a new git repos service
func NewService(settings *config.GitReposSettings) (*Service, error)

// Initialize prepares the service (sync leader/follower logic)
func (s *Service) Initialize(ctx context.Context) error

// SyncAll synchronizes all configured repositories
func (s *Service) SyncAll(ctx context.Context) error

// syncRepo syncs a single repository
func (s *Service) syncRepo(ctx context.Context, repoID, url string) error

// IsReady returns true if indexes are ready for search
func (s *Service) IsReady() bool

// GetIndexAlias returns the combined index for searching
func (s *Service) GetIndexAlias() (bleve.IndexAlias, error)

// Close releases resources
func (s *Service) Close() error
```

**Leader/Follower Logic**:
```go
func (s *Service) Initialize(ctx context.Context) error {
    // Try to become sync leader
    acquired, err := s.lock.TryLock()
    if err != nil {
        return err
    }

    if acquired {
        // Leader: sync repos
        defer s.lock.Unlock()
        if err := s.SyncAll(ctx); err != nil {
            slog.Error("Sync failed", "error", err)
            // Continue to open indexes anyway
        }
    } else {
        // Follower: wait for sync to complete
        if err := s.lock.Lock(s.settings.SyncTimeout); err != nil {
            slog.Warn("Timeout waiting for sync, using existing indexes")
        }
        s.lock.Unlock()
    }

    // Open indexes read-only
    return s.openIndexes()
}
```

**Unit Tests**:
1. Test `NewService` initializes all components
2. Test `NewService` creates base directory structure
3. Test `Initialize` as leader performs sync
4. Test `Initialize` as follower waits for sync
5. Test `SyncAll` processes all repos in parallel
6. Test `SyncAll` limits parallelism to 4
7. Test `syncRepo` clones new repository
8. Test `syncRepo` fetches existing repository
9. Test `syncRepo` triggers reindex when commit changes
10. Test `syncRepo` skips reindex when commit unchanged
11. Test `syncRepo` isolates errors per repository
12. Test `IsReady` returns correct state
13. Test `Close` releases all resources

**Commit Message**: `feat(gitrepos): add service coordinating sync and indexing`

---

## Task 10: Search Tool Handler

**Package**: `internal/gitrepos`

**Files to Create**:
- `internal/gitrepos/tools_search.go` - Search tool implementation
- `internal/gitrepos/tools_search_test.go` - Tests

**Implementation Details**:

```go
// SearchArgument defines search parameters
type SearchArgument struct {
    Query      string `json:"query" jsonschema_description:"Search query"`
    Repository string `json:"repository,omitempty" jsonschema_description:"Filter by repository name"`
    Extension  string `json:"extension,omitempty" jsonschema_description:"Filter by file extension"`
}

// SearchHandler handles the search MCP tool
type SearchHandler struct {
    service *Service
}

// NewSearchHandler creates a new search handler
func NewSearchHandler(service *Service) *SearchHandler

// Handle executes the search and returns formatted results
func (h *SearchHandler) Handle(ctx context.Context, args SearchArgument) (*mcp.CallToolResult, error)

// formatResults formats Bleve search results for MCP response
func (h *SearchHandler) formatResults(results *bleve.SearchResult, query string) *mcp.CallToolResult

// GetToolDefinition returns the MCP tool definition
func (h *SearchHandler) GetToolDefinition() mcp.Tool
```

**Unit Tests**:
1. Test search with simple query
2. Test search with repository filter
3. Test search with extension filter
4. Test search with both filters
5. Test search with no results returns helpful message
6. Test search when service not ready returns appropriate message
7. Test search respects MaxResults limit
8. Test result formatting includes repository, path, and snippet
9. Test `GetToolDefinition` returns correct schema

**Commit Message**: `feat(gitrepos): add search tool handler for MCP`

---

## Task 11: Read Tool Handler

**Package**: `internal/gitrepos`

**Files to Create**:
- `internal/gitrepos/tools_read.go` - Read tool implementation
- `internal/gitrepos/tools_read_test.go` - Tests

**Implementation Details**:

```go
// ReadArgument defines read parameters
type ReadArgument struct {
    Repository string `json:"repository" jsonschema_description:"Repository name"`
    Path       string `json:"path" jsonschema_description:"File path relative to repository root"`
}

// ReadHandler handles the read MCP tool
type ReadHandler struct {
    service *Service
}

// NewReadHandler creates a new read handler
func NewReadHandler(service *Service) *ReadHandler

// Handle reads a file and returns formatted content
func (h *ReadHandler) Handle(ctx context.Context, args ReadArgument) (*mcp.CallToolResult, error)

// GetToolDefinition returns the MCP tool definition
func (h *ReadHandler) GetToolDefinition() mcp.Tool

// extensionToLanguage maps file extension to language hint
func extensionToLanguage(ext string) string
```

**Security Validations**:
1. Clean path using `filepath.Clean()`
2. Reject absolute paths
3. Reject paths starting with `..`
4. Verify final path is within repo directory
5. Check file exists and is not directory
6. Check file size against limit
7. Check for binary content

**Unit Tests**:
1. Test reading valid file returns content
2. Test reading non-existent repository returns error
3. Test reading non-existent file returns error
4. Test path traversal with `../` is rejected
5. Test path traversal with absolute path is rejected
6. Test path traversal with encoded characters is handled
7. Test reading directory returns error
8. Test reading file over size limit returns error
9. Test reading binary file returns error
10. Test `extensionToLanguage` for common extensions
11. Test response formatting includes language hint
12. Test `GetToolDefinition` returns correct schema

**Commit Message**: `feat(gitrepos): add read tool handler for MCP`

---

## Task 12: MCP Tool Registration

**Package**: `internal/mcp`

**Files to Modify**:
- `internal/mcp/server.go` - Add tool registration
- `internal/mcp/server_test.go` - Add tests

**Implementation Details**:

```go
// ServerConfig extended with git repos service
type ServerConfig struct {
    Name        string
    Version     string
    GitReposSvc *gitrepos.Service // Optional, nil if disabled
}

// CreateServer creates MCP server with tools conditionally registered
func CreateServer(cfg *ServerConfig) (*server.MCPServer, error) {
    srv := server.NewMCPServer(cfg.Name, cfg.Version)

    // Register git repos tools if service is provided
    if cfg.GitReposSvc != nil {
        searchHandler := gitrepos.NewSearchHandler(cfg.GitReposSvc)
        readHandler := gitrepos.NewReadHandler(cfg.GitReposSvc)

        srv.AddTool(searchHandler.GetToolDefinition(), searchHandler.Handle)
        srv.AddTool(readHandler.GetToolDefinition(), readHandler.Handle)
    }

    return srv, nil
}
```

**Modify app/runner.go**:
```go
func RunWithDeps(params RunParams, flags *pflag.FlagSet) error {
    settings, err := params.LoadSettings(flags)
    // ...

    var gitReposSvc *gitrepos.Service
    if settings.GitRepos.Enabled {
        gitReposSvc, err = gitrepos.NewService(&settings.GitRepos)
        if err != nil {
            return err
        }
        defer gitReposSvc.Close()

        if err := gitReposSvc.Initialize(ctx); err != nil {
            slog.Error("Git repos initialization failed", "error", err)
            // Continue without git repos
            gitReposSvc = nil
        }
    }

    mcpServer, err := params.CreateServer(&mcp.ServerConfig{
        Name:        "relic-mcp",
        Version:     Version,
        GitReposSvc: gitReposSvc,
    })
    // ...
}
```

**Unit Tests**:
1. Test server creation without git repos service (no tools registered)
2. Test server creation with git repos service (tools registered)
3. Test search tool is accessible via MCP protocol
4. Test read tool is accessible via MCP protocol

**Commit Message**: `feat(mcp): register search and read tools conditionally`

---

## Task 13: Integration Tests

**Package**: `tests/integration`

**Files to Create**:
- `tests/integration/gitrepos_test.go` - Full integration tests

**Test Setup**:
- Create temporary directory with test repositories
- Use small test repos (can be created during test setup)
- Clean up after tests

**Integration Tests**:

### 13.1 Service Lifecycle Tests
1. Test service initializes correctly with valid config
2. Test service handles disabled config (no-op)
3. Test service creates directory structure
4. Test service leader election with concurrent starts
5. Test service graceful shutdown

### 13.2 Sync Tests
1. Test initial clone of repository
2. Test subsequent fetch and update
3. Test sync timeout handling
4. Test sync with invalid repository URL
5. Test sync continues after single repo failure

### 13.3 Index Tests
1. Test full indexing creates searchable index
2. Test incremental indexing updates correctly
3. Test index alias combines multiple repos

### 13.4 Search Tool MCP Tests
1. Test search via MCP protocol returns results
2. Test search with repository filter via MCP
3. Test search with extension filter via MCP
4. Test search with no matches returns helpful message
5. Test search when indexes not ready

### 13.5 Read Tool MCP Tests
1. Test read file via MCP protocol
2. Test read with invalid repo returns error via MCP
3. Test read with invalid path returns error via MCP
4. Test read with path traversal attempt returns error via MCP
5. Test read binary file returns error via MCP

### 13.6 SSE Transport Tests
1. Test search tool accessible via SSE endpoint
2. Test read tool accessible via SSE endpoint
3. Test tools with authentication enabled

**Test Utilities to Add to testkit**:
```go
// CreateTestRepo creates a git repository with sample files for testing
func CreateTestRepo(t *testing.T, dir string, files map[string]string) string

// StartTestMCPServer starts an MCP server with git repos enabled for testing
func StartTestMCPServer(t *testing.T, repoURLs []string) (*Service, func())
```

**Commit Message**: `test: add comprehensive integration tests for git repos`

---

## Implementation Order

Execute tasks in the following order, committing after each passes `make lint test`:

| Order | Task | Est. Files | Dependencies |
|-------|------|------------|--------------|
| 1 | Task 1: GitReposSettings | 2 | None |
| 2 | Task 2: Domain Models | 2 | None |
| 3 | Task 3: URL Utilities | 2 | None |
| 4 | Task 4: File Lock | 2 | None |
| 5 | Task 7: File Filtering | 2 | None |
| 6 | Task 5: Git Operations | 2 | Task 3 |
| 7 | Task 6: Manifest | 2 | Task 3 |
| 8 | Task 8: Bleve Indexer | 2 | Task 2, 7 |
| 9 | Task 9: Service | 2 | Task 4-8 |
| 10 | Task 10: Search Tool | 2 | Task 2, 9 |
| 11 | Task 11: Read Tool | 2 | Task 9 |
| 12 | Task 12: MCP Registration | 2 | Task 10-11 |
| 13 | Task 13: Integration Tests | 2 | All |

---

## Test Coverage Requirements

Each task must achieve 100% test coverage for new code. Use the following to verify:

```bash
# Check coverage for specific package
go test -coverprofile=coverage.out ./internal/gitrepos/...
go tool cover -func=coverage.out

# Generate HTML report
go tool cover -html=coverage.out -o coverage.html
```

---

## Validation Checklist

Before committing each task:

- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] New code has 100% test coverage
- [ ] No hardcoded paths or credentials
- [ ] Error messages are user-friendly
- [ ] Logging uses structured `slog`
- [ ] Tests use table-driven patterns where appropriate
- [ ] Integration tests clean up test data
