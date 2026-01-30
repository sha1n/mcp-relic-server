# Git Repository Code Search Specification

This specification describes the design for Git repository indexing and code search in the RELIC MCP Server.

## 1. Overview

### 1.1 Goals
- **Memory Efficient**: Minimize heap allocations; leverage OS page cache via mmap'd indexes
- **Fast**: Incremental indexing; parallel operations where safe
- **Robust**: Graceful degradation; per-repo error isolation
- **Simple Operations**: No background daemons; coordination via file locks

### 1.2 Assumptions
- Users have SSH configured for all repositories (`~/.ssh/config`, ssh-agent, etc.)
- Repositories are cloned via SSH URLs (e.g., `git@github.com:org/repo.git`)
- This feature is **opt-in** and disabled by default

### 1.3 Non-Goals
- Real-time file watching (use `git pull` based refresh)
- Semantic code analysis (AST parsing, symbol extraction)
- Multi-branch indexing (default branch only)

---

## 2. Architecture

### 2.1 Storage Layout

```
~/.relic-mcp/
├── repos/
│   ├── github.com_org_repo1/       # Shallow clone
│   │   ├── .git/
│   │   └── ...
│   └── github.com_org_repo2/
├── indexes/
│   ├── github.com_org_repo1.bleve/ # Disk-based Bleve index
│   └── github.com_org_repo2.bleve/
├── state/
│   ├── sync.lock                   # flock-based coordination
│   └── manifest.json               # Repo metadata & last sync info
└── logs/
    └── sync.log                    # Rotating sync log
```

### 2.2 Process Coordination Model

Since `stdio` transport spawns one server per agent, multiple instances may run concurrently. We use a **Leader-Follower** model with proper file locking.

```
┌─────────────────────────────────────────────────────────────┐
│                        Startup Flow                          │
├─────────────────────────────────────────────────────────────┤
│  1. Try flock(sync.lock, LOCK_EX | LOCK_NB)                 │
│     ├─ Success → This instance is SYNC LEADER               │
│     │            - Perform git pull (if due)                │
│     │            - Reindex changed repos                    │
│     │            - Update manifest.json                     │
│     │            - Release lock                             │
│     │            - Open indexes read-only                   │
│     │                                                       │
│     └─ EWOULDBLOCK → This instance is FOLLOWER              │
│                      - Wait for lock release (with timeout) │
│                      - Open indexes read-only               │
└─────────────────────────────────────────────────────────────┘
```

**Key Properties:**
- Lock is held only during sync operations (seconds to minutes)
- Followers wait with timeout, then proceed with potentially stale index
- Lock automatically releases if process crashes (OS behavior)
- No stale lock files possible with `flock(2)`

### 2.3 Index Sharing via mmap

All instances open Bleve indexes in **read-only mode** after sync completes. BoltDB (Bleve's backend) uses mmap, allowing the OS to share physical memory pages across processes.

```
┌──────────────┐  ┌──────────────┐  ┌──────────────┐
│  Instance 1  │  │  Instance 2  │  │  Instance 3  │
│  (Follower)  │  │  (Follower)  │  │  (Follower)  │
└──────┬───────┘  └──────┬───────┘  └──────┬───────┘
       │                 │                 │
       └────────────┬────┴────────┬────────┘
                    │             │
                    ▼             ▼
            ┌───────────────────────────┐
            │   OS Page Cache (mmap)    │
            │   ┌───────────────────┐   │
            │   │  repo1.bleve      │   │  ← Single copy in RAM
            │   │  repo2.bleve      │   │
            │   └───────────────────┘   │
            └───────────────────────────┘
                         │
                         ▼
            ┌───────────────────────────┐
            │      Disk Storage         │
            └───────────────────────────┘
```

---

## 3. Configuration

### 3.1 Settings

| Environment Variable | CLI Flag | Description | Default |
|---------------------|----------|-------------|---------|
| `RELIC_MCP_GIT_REPOS_ENABLED` | `--git-repos-enabled` | Enable git repository indexing | `false` |
| `RELIC_MCP_GIT_REPOS_URLS` | `--git-repos-urls` | Comma-separated SSH URLs | (none) |
| `RELIC_MCP_GIT_REPOS_BASE_DIR` | `--git-repos-base-dir` | Base directory for all git data | `~/.relic-mcp` |
| `RELIC_MCP_GIT_REPOS_SYNC_INTERVAL` | `--git-repos-sync-interval` | Min interval between syncs | `15m` |
| `RELIC_MCP_GIT_REPOS_SYNC_TIMEOUT` | `--git-repos-sync-timeout` | Max time to wait for sync lock | `60s` |
| `RELIC_MCP_GIT_REPOS_MAX_FILE_SIZE` | `--git-repos-max-file-size` | Skip files larger than this | `256KB` |
| `RELIC_MCP_GIT_REPOS_MAX_RESULTS` | `--git-repos-max-results` | Max search results | `20` |

### 3.2 File Filtering

**Default exclusions** (not configurable in v1, hardcoded for simplicity):

```go
var DefaultExcludePatterns = []string{
    // Dependencies
    "node_modules/**", "vendor/**", "venv/**", ".venv/**",
    "target/**", "build/**", "dist/**", "out/**",
    // Generated
    "*.min.js", "*.min.css", "*.map", "*.pb.go",
    "package-lock.json", "yarn.lock", "pnpm-lock.yaml",
    "go.sum", "poetry.lock", "Cargo.lock",
    // Binary/Media (also detected via content)
    "*.png", "*.jpg", "*.jpeg", "*.gif", "*.ico", "*.svg",
    "*.woff", "*.woff2", "*.ttf", "*.eot",
    "*.zip", "*.tar", "*.gz", "*.rar",
    "*.exe", "*.dll", "*.so", "*.dylib",
    "*.pdf", "*.doc", "*.docx",
}
```

**Binary detection**: Files are skipped if the first 512 bytes contain null bytes.

### 3.3 Config Struct

```go
// GitReposSettings in internal/config/settings.go
type GitReposSettings struct {
    Enabled      bool          `mapstructure:"enabled"`
    URLs         []string      `mapstructure:"urls"`
    BaseDir      string        `mapstructure:"base_dir"`
    SyncInterval time.Duration `mapstructure:"sync_interval"`
    SyncTimeout  time.Duration `mapstructure:"sync_timeout"`
    MaxFileSize  int64         `mapstructure:"max_file_size"`  // bytes
    MaxResults   int           `mapstructure:"max_results"`
}
```

---

## 4. Data Model

### 4.1 Indexed Document Schema

```go
// internal/domain/code.go

// CodeDocument represents an indexed source file
type CodeDocument struct {
    // Unique identifier: "github.com_org_repo/path/to/file.go"
    ID string `json:"id"`

    // Repository identifier (derived from URL)
    // e.g., "github.com/org/repo"
    Repository string `json:"repository"`

    // File path relative to repo root
    // e.g., "src/main/java/App.java"
    FilePath string `json:"file_path"`

    // File extension without dot
    // e.g., "java", "go", "py"
    Extension string `json:"extension"`

    // File content (for indexing and snippets)
    Content string `json:"content"`
}

// Bleve field constants
const (
    CodeFieldID         = "id"
    CodeFieldRepository = "repository"
    CodeFieldFilePath   = "file_path"
    CodeFieldExtension  = "extension"
    CodeFieldContent    = "content"
)
```

### 4.2 Manifest Schema

```go
// internal/gitrepos/manifest.go

type Manifest struct {
    Version    int                   `json:"version"`     // Schema version (1)
    LastSync   time.Time             `json:"last_sync"`
    Repos      map[string]RepoState  `json:"repos"`       // key: repo ID
}

type RepoState struct {
    URL         string    `json:"url"`
    ClonedAt    time.Time `json:"cloned_at"`
    LastPull    time.Time `json:"last_pull"`
    LastCommit  string    `json:"last_commit"`   // HEAD commit SHA
    LastIndexed string    `json:"last_indexed"`  // Indexed commit SHA
    FileCount   int       `json:"file_count"`
    Error       string    `json:"error,omitempty"` // Last error, if any
}
```

---

## 5. Sync Workflow

### 5.1 Sync Leader Flow

```
SyncLeader():
    1. Acquire flock(sync.lock, LOCK_EX)

    2. Load manifest.json (or create if missing)

    3. For each configured repo URL (parallel, max 4):
        a. repoID = urlToRepoID(url)  // "github.com_org_repo"
        b. repoDir = baseDir/repos/{repoID}

        c. If repoDir doesn't exist:
            - git clone --depth 1 --single-branch {url} {repoDir}
            - manifest.repos[repoID] = new RepoState

        d. Else:
            - git -C {repoDir} fetch --depth 1
            - git -C {repoDir} reset --hard origin/HEAD

        e. newCommit = git -C {repoDir} rev-parse HEAD

        f. If newCommit != manifest.repos[repoID].LastIndexed:
            - IndexRepository(repoID, repoDir)
            - manifest.repos[repoID].LastIndexed = newCommit

        g. Update manifest.repos[repoID] timestamps

    4. Remove repos from manifest that are no longer in config

    5. Save manifest.json

    6. Release flock
```

### 5.2 Incremental Indexing Strategy

For **speed**, we use a hybrid approach:

1. **First clone**: Full index of all files
2. **Subsequent pulls**:
   - If `git diff --name-only oldCommit..newCommit` returns < 100 files → incremental update
   - Otherwise → full reindex

```go
func (s *GitRepoService) IndexRepository(repoID, repoDir string) error {
    oldCommit := s.manifest.Repos[repoID].LastIndexed
    newCommit := getCurrentCommit(repoDir)

    if oldCommit == "" {
        return s.fullIndex(repoID, repoDir)
    }

    changedFiles, err := getChangedFiles(repoDir, oldCommit, newCommit)
    if err != nil || len(changedFiles) > 100 {
        return s.fullIndex(repoID, repoDir)
    }

    return s.incrementalIndex(repoID, repoDir, changedFiles)
}
```

### 5.3 Memory-Efficient File Processing

```go
// Stream files without loading entire repo into memory
func (s *GitRepoService) fullIndex(repoID, repoDir string) error {
    index, err := s.openIndexForWrite(repoID)
    if err != nil {
        return err
    }
    defer index.Close()

    batch := index.NewBatch()
    batchSize := 0
    const maxBatchSize = 100
    const maxBatchBytes = 10 * 1024 * 1024 // 10MB
    batchBytes := 0

    err = filepath.WalkDir(repoDir, func(path string, d fs.DirEntry, err error) error {
        if err != nil || d.IsDir() {
            return err
        }

        relPath, _ := filepath.Rel(repoDir, path)

        // Skip .git directory
        if strings.HasPrefix(relPath, ".git") {
            return filepath.SkipDir
        }

        // Skip excluded patterns
        if s.shouldExclude(relPath) {
            return nil
        }

        // Skip large files
        info, _ := d.Info()
        if info.Size() > s.settings.MaxFileSize {
            return nil
        }

        // Read and check for binary
        content, err := os.ReadFile(path)
        if err != nil || isBinary(content) {
            return nil
        }

        doc := CodeDocument{
            ID:         repoID + "/" + relPath,
            Repository: repoIDToDisplay(repoID),
            FilePath:   relPath,
            Extension:  strings.TrimPrefix(filepath.Ext(relPath), "."),
            Content:    string(content),
        }

        batch.Index(doc.ID, doc)
        batchSize++
        batchBytes += len(content)

        // Flush batch to control memory
        if batchSize >= maxBatchSize || batchBytes >= maxBatchBytes {
            if err := index.Batch(batch); err != nil {
                return err
            }
            batch = index.NewBatch()
            batchSize = 0
            batchBytes = 0
        }

        return nil
    })

    // Flush remaining
    if batchSize > 0 {
        return index.Batch(batch)
    }
    return err
}

func isBinary(content []byte) bool {
    checkLen := min(512, len(content))
    for i := 0; i < checkLen; i++ {
        if content[i] == 0 {
            return true
        }
    }
    return false
}
```

---

## 6. MCP Tools

### 6.1 Tool Definitions

```go
// internal/mcp/tools_code.go

type SearchArgument struct {
    Query      string `json:"query" jsonschema_description:"Search query. Use natural language or keywords."`
    Repository string `json:"repository,omitempty" jsonschema_description:"Filter by repository name (substring match)"`
    Extension  string `json:"extension,omitempty" jsonschema_description:"Filter by file extension (e.g., 'go', 'py', 'java')"`
}

type ReadArgument struct {
    Repository string `json:"repository" jsonschema_description:"Repository name (e.g., 'github.com/org/repo')"`
    Path       string `json:"path" jsonschema_description:"File path relative to repository root (e.g., 'src/main.go')"`
}
```

### 6.2 Tool Metadata

```yaml
# search - Default description (can be overridden in mcp-metadata.yaml)
name: search
description: |
  Search across indexed git repositories for code, documentation, and configuration.

  WHEN TO USE: Use this to find implementation patterns, understand how features work
  across the codebase, locate configuration files, or find usage examples.

  HOW IT WORKS: Searches file content with optional filtering by repository or
  file extension. Returns matching files with relevant code snippets.
```

```yaml
# read - Default description (can be overridden in mcp-metadata.yaml)
name: read
description: |
  Read the full content of a file from an indexed git repository.

  WHEN TO USE: Use after search to retrieve the complete file content,
  or when you know the exact repository and file path you need to read.

  HOW IT WORKS: Provide the repository name and file path. Returns the full
  file content with syntax highlighting hints based on file extension.
```

### 6.3 `search` Implementation

```go
func (h *SearchHandler) Handle(ctx context.Context, args SearchArgument) (*mcp.CallToolResult, error) {
    // Build query
    contentQuery := bleve.NewMatchQuery(args.Query)
    contentQuery.SetField(CodeFieldContent)
    contentQuery.SetFuzziness(1)

    var query query.Query = contentQuery

    // Apply filters
    if args.Repository != "" || args.Extension != "" {
        queries := []query.Query{contentQuery}

        if args.Repository != "" {
            repoQuery := bleve.NewWildcardQuery("*" + args.Repository + "*")
            repoQuery.SetField(CodeFieldRepository)
            queries = append(queries, repoQuery)
        }

        if args.Extension != "" {
            extQuery := bleve.NewTermQuery(strings.ToLower(args.Extension))
            extQuery.SetField(CodeFieldExtension)
            queries = append(queries, extQuery)
        }

        query = bleve.NewConjunctionQuery(queries...)
    }

    // Execute search
    searchReq := bleve.NewSearchRequest(query)
    searchReq.Size = h.settings.MaxResults
    searchReq.Fields = []string{CodeFieldRepository, CodeFieldFilePath, CodeFieldExtension}
    searchReq.Highlight = bleve.NewHighlightWithStyle("ansi")
    searchReq.Highlight.AddField(CodeFieldContent)

    results, err := h.indexAlias.Search(searchReq)
    if err != nil {
        return nil, err
    }

    return h.formatResults(results), nil
}
```

### 6.4 Result Format

```markdown
Found 3 results for "handleAuth middleware":

**1. github.com/org/api-server** `src/middleware/auth.go`
```go
func handleAuth(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        token := r.Header.Get("Authorization")
        // ... highlighted match ...
    })
}
```

**2. github.com/org/api-server** `src/middleware/auth_test.go`
```go
func TestHandleAuth_ValidToken(t *testing.T) {
    // ... highlighted match ...
}
```

**3. github.com/org/shared-lib** `pkg/auth/middleware.go`
```go
// HandleAuth creates authentication middleware
// ... highlighted match ...
```
```

### 6.5 `read` Implementation

```go
func (h *ReadHandler) Handle(ctx context.Context, args ReadArgument) (*mcp.CallToolResult, error) {
    // Validate repository exists
    repoID := displayToRepoID(args.Repository)  // "github.com/org/repo" -> "github.com_org_repo"
    repoDir := filepath.Join(h.settings.BaseDir, "repos", repoID)

    if _, err := os.Stat(repoDir); os.IsNotExist(err) {
        return nil, fmt.Errorf("repository not found: %s", args.Repository)
    }

    // Sanitize and validate path (prevent directory traversal)
    cleanPath := filepath.Clean(args.Path)
    if strings.HasPrefix(cleanPath, "..") || filepath.IsAbs(cleanPath) {
        return nil, fmt.Errorf("invalid path: %s", args.Path)
    }

    // Construct full path
    fullPath := filepath.Join(repoDir, cleanPath)

    // Ensure path is still within repo directory
    if !strings.HasPrefix(fullPath, repoDir) {
        return nil, fmt.Errorf("path escapes repository: %s", args.Path)
    }

    // Check file exists and is not a directory
    info, err := os.Stat(fullPath)
    if os.IsNotExist(err) {
        return nil, fmt.Errorf("file not found: %s in %s", args.Path, args.Repository)
    }
    if info.IsDir() {
        return nil, fmt.Errorf("path is a directory, not a file: %s", args.Path)
    }

    // Check file size
    if info.Size() > h.settings.MaxFileSize {
        return nil, fmt.Errorf("file too large (%d bytes, max %d): %s",
            info.Size(), h.settings.MaxFileSize, args.Path)
    }

    // Read file content
    content, err := os.ReadFile(fullPath)
    if err != nil {
        return nil, fmt.Errorf("failed to read file: %w", err)
    }

    // Check for binary content
    if isBinary(content) {
        return nil, fmt.Errorf("file appears to be binary: %s", args.Path)
    }

    // Format response with language hint
    ext := strings.TrimPrefix(filepath.Ext(args.Path), ".")
    lang := extensionToLanguage(ext)

    return &mcp.CallToolResult{
        Content: []mcp.Content{
            &mcp.TextContent{
                Text: formatCodeResponse(args.Repository, args.Path, lang, string(content)),
            },
        },
    }, nil
}

func formatCodeResponse(repo, path, lang, content string) string {
    var sb strings.Builder
    sb.WriteString(fmt.Sprintf("**%s** `%s`\n\n", repo, path))
    sb.WriteString(fmt.Sprintf("```%s\n", lang))
    sb.WriteString(content)
    if !strings.HasSuffix(content, "\n") {
        sb.WriteString("\n")
    }
    sb.WriteString("```\n")
    return sb.String()
}

func extensionToLanguage(ext string) string {
    languages := map[string]string{
        "go":    "go",
        "py":    "python",
        "js":    "javascript",
        "ts":    "typescript",
        "tsx":   "tsx",
        "jsx":   "jsx",
        "java":  "java",
        "kt":    "kotlin",
        "rs":    "rust",
        "rb":    "ruby",
        "php":   "php",
        "cs":    "csharp",
        "cpp":   "cpp",
        "c":     "c",
        "h":     "c",
        "hpp":   "cpp",
        "swift": "swift",
        "scala": "scala",
        "sh":    "bash",
        "bash":  "bash",
        "zsh":   "zsh",
        "yaml":  "yaml",
        "yml":   "yaml",
        "json":  "json",
        "xml":   "xml",
        "html":  "html",
        "css":   "css",
        "scss":  "scss",
        "sql":   "sql",
        "md":    "markdown",
        "proto": "protobuf",
        "tf":    "hcl",
        "toml":  "toml",
    }
    if lang, ok := languages[strings.ToLower(ext)]; ok {
        return lang
    }
    return ext
}
```

### 6.6 `read` Result Format

```markdown
**github.com/org/api-server** `src/middleware/auth.go`

```go
package middleware

import (
    "net/http"
    "strings"
)

// HandleAuth creates authentication middleware that validates
// JWT tokens from the Authorization header.
func HandleAuth(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        token := r.Header.Get("Authorization")
        if token == "" {
            http.Error(w, "unauthorized", http.StatusUnauthorized)
            return
        }

        // Strip "Bearer " prefix
        token = strings.TrimPrefix(token, "Bearer ")

        claims, err := validateToken(token)
        if err != nil {
            http.Error(w, "invalid token", http.StatusUnauthorized)
            return
        }

        ctx := context.WithValue(r.Context(), userClaimsKey, claims)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```
```

---

## 7. Package Structure

```
internal/
├── gitrepos/
│   ├── service.go       # Main service: sync, index management
│   ├── git.go           # Git operations (clone, pull, diff)
│   ├── indexer.go       # Bleve indexing logic
│   ├── manifest.go      # Manifest read/write
│   ├── filelock.go      # flock wrapper
│   ├── filter.go        # File filtering (patterns, binary detection)
│   ├── tools.go         # MCP tool handlers (search, read)
│   └── service_test.go
└── config/
    └── settings.go      # Add GitReposSettings
```

---

## 8. Error Handling

### 8.1 Per-Repo Isolation

Errors in one repository don't affect others:

```go
func (s *GitRepoService) SyncAll(ctx context.Context) error {
    var wg sync.WaitGroup
    semaphore := make(chan struct{}, 4) // Max 4 parallel

    for _, url := range s.settings.URLs {
        wg.Add(1)
        go func(url string) {
            defer wg.Done()
            semaphore <- struct{}{}
            defer func() { <-semaphore }()

            repoID := urlToRepoID(url)
            if err := s.syncRepo(ctx, repoID, url); err != nil {
                slog.Error("Failed to sync repo", "repo", repoID, "error", err)
                s.manifest.Repos[repoID].Error = err.Error()
                // Continue with other repos
            } else {
                s.manifest.Repos[repoID].Error = ""
            }
        }(url)
    }

    wg.Wait()
    return nil // Individual errors logged, not propagated
}
```

### 8.2 Graceful Degradation

| Scenario | Behavior |
|----------|----------|
| Sync lock timeout | Log warning, use existing index (may be stale) |
| Git clone/pull fails | Mark repo as errored in manifest, skip indexing |
| Index open fails | Exclude repo from search, log error |
| Search with no indexes | Return empty results with message |

### 8.3 User Feedback

The `search` tool returns helpful messages:

```go
if len(s.settings.URLs) == 0 {
    return "Code search is not configured. Set RELIC_MCP_GIT_REPOS_URLS to enable."
}

if !s.indexReady {
    return "Code indexes are being built. Please try again shortly."
}

if len(results) == 0 {
    return fmt.Sprintf("No results found for '%s'. Try different keywords or remove filters.", args.Query)
}
```

The `read` tool returns clear error messages:

```go
// Repository not found
return nil, fmt.Errorf("repository not found: %s", args.Repository)

// File not found
return nil, fmt.Errorf("file not found: %s in %s", args.Path, args.Repository)

// Path traversal attempt
return nil, fmt.Errorf("invalid path: %s", args.Path)

// Binary file
return nil, fmt.Errorf("file appears to be binary: %s", args.Path)

// File too large
return nil, fmt.Errorf("file too large (%d bytes, max %d): %s", size, max, args.Path)
```

---

## 9. Security Considerations

### 9.1 Path Traversal Prevention

The `read` tool must prevent directory traversal attacks:

```go
// 1. Clean the path (removes .., redundant slashes)
cleanPath := filepath.Clean(args.Path)

// 2. Reject absolute paths and paths starting with ..
if strings.HasPrefix(cleanPath, "..") || filepath.IsAbs(cleanPath) {
    return nil, fmt.Errorf("invalid path: %s", args.Path)
}

// 3. Verify final path is within repo directory
fullPath := filepath.Join(repoDir, cleanPath)
if !strings.HasPrefix(fullPath, repoDir) {
    return nil, fmt.Errorf("path escapes repository: %s", args.Path)
}
```

### 9.2 File Access Restrictions

| Restriction | Rationale |
|-------------|-----------|
| Only files within cloned repos | Prevents access to system files |
| Max file size enforced | Prevents memory exhaustion |
| Binary files rejected | Prevents serving executables or sensitive binaries |
| `.git` directory excluded from indexing | Prevents exposing git metadata |

### 9.3 Repository Access

- Repositories are cloned using the user's SSH configuration
- No credentials are stored by the server
- Only explicitly configured repository URLs are cloned

---

## 10. Performance Characteristics

### 10.1 Memory Usage

| Component | Memory Strategy |
|-----------|----------------|
| Git clones | Shallow (`--depth 1`), single branch |
| File reading | Stream via `filepath.WalkDir`, batch flush at 10MB |
| Bleve indexes | Disk-based, mmap'd, shared across processes |
| Search results | Limited to `MaxResults` (default 20) |

**Expected heap usage per instance**: ~50-100MB (primarily Go runtime + Bleve query buffers)

### 10.2 Disk Usage

| Data | Approximate Size |
|------|------------------|
| Shallow clone | ~10-30% of full repo |
| Bleve index | ~50-100% of indexed text size |

### 10.3 Sync Performance

| Operation | Expected Duration |
|-----------|-------------------|
| Initial clone (medium repo) | 5-30 seconds |
| Git fetch + reset | 1-5 seconds |
| Full reindex (10k files) | 10-30 seconds |
| Incremental reindex (<100 files) | 1-5 seconds |

---

## 11. Implementation Phases

### Phase 1: Core Infrastructure
- [ ] Add `GitReposSettings` to config
- [ ] Implement `filelock.go` using `syscall.Flock`
- [ ] Implement `manifest.go` for state persistence
- [ ] Add base directory structure creation

### Phase 2: Git Operations
- [ ] Implement `git.go` with clone, fetch, reset, diff
- [ ] Add SSH URL parsing and repo ID generation
- [ ] Implement parallel sync with semaphore

### Phase 3: Indexing
- [ ] Implement `filter.go` with pattern matching and binary detection
- [ ] Implement `indexer.go` with batched, memory-efficient indexing
- [ ] Add incremental indexing support
- [ ] Create `IndexAlias` for multi-repo search

### Phase 4: MCP Integration
- [ ] Implement `search` tool handler with result formatting
- [ ] Implement `read` tool handler with path validation
- [ ] Register tools conditionally (only when enabled)
- [ ] Add tool metadata defaults

### Phase 5: Testing & Documentation
- [ ] Unit tests for each component
- [ ] Integration tests with testkit
- [ ] Update README and configuration docs
- [ ] Add example configurations

---

## 12. Future Considerations (Out of Scope for v1)

- **Configurable exclude patterns**: Allow users to add custom patterns
- **Multi-branch support**: Index specific branches per repo
- **Webhook-triggered sync**: For SSE transport, accept webhook to trigger immediate sync
- **Language-aware search**: Boost matches in function names, class definitions
- **Repository groups**: Logical groupings with separate search scopes
