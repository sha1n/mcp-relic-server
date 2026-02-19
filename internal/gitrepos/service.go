package gitrepos

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/sha1n/mcp-relic-server/internal/config"
)

const (
	// LockFilename is the name of the sync lock file
	LockFilename = "sync.lock"

	// MaxParallelSyncs is the maximum number of concurrent repository syncs
	MaxParallelSyncs = 4
)

// Service coordinates git operations, indexing, and search.
type Service struct {
	settings *config.GitReposSettings
	git      GitOperations
	indexer  IndexOperations
	manifest ManifestOperations
	lock     SyncLock
	alias    bleve.IndexAlias
	ready    bool
	mu       sync.RWMutex
}

// ServiceDeps holds injectable dependencies for creating a Service.
type ServiceDeps struct {
	Git      GitOperations
	Indexer  IndexOperations
	Manifest ManifestOperations
	Lock     SyncLock
}

// NewService creates a new git repos service.
func NewService(settings *config.GitReposSettings) (*Service, error) {
	if settings == nil {
		return nil, fmt.Errorf("settings cannot be nil")
	}

	// Ensure base directory exists
	if err := os.MkdirAll(settings.BaseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base directory: %w", err)
	}

	// Create repos directory
	reposDir := filepath.Join(settings.BaseDir, "repos")
	if err := os.MkdirAll(reposDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create repos directory: %w", err)
	}

	// Create indexes directory
	indexesDir := filepath.Join(settings.BaseDir, "indexes")
	if err := os.MkdirAll(indexesDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create indexes directory: %w", err)
	}

	// Load or create manifest
	manifestPath := filepath.Join(settings.BaseDir, ManifestFilename)
	manifest, err := LoadManifest(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load manifest: %w", err)
	}

	// Create components
	filter := NewFileFilter(settings.MaxFileSize)
	indexer := NewIndexer(settings.BaseDir, filter, settings.MaxFileSize)
	lock := NewFileLock(filepath.Join(settings.BaseDir, LockFilename))
	git := NewGitClient()

	return &Service{
		settings: settings,
		git:      git,
		indexer:  indexer,
		manifest: manifest,
		lock:     lock,
	}, nil
}

// NewServiceWithDeps creates a Service with injected dependencies for testing.
func NewServiceWithDeps(settings *config.GitReposSettings, deps ServiceDeps) *Service {
	return &Service{
		settings: settings,
		git:      deps.Git,
		indexer:  deps.Indexer,
		manifest: deps.Manifest,
		lock:     deps.Lock,
	}
}

// Initialize prepares the service with leader/follower sync logic.
func (s *Service) Initialize(ctx context.Context) error {
	acquired, err := s.lock.TryLock()
	if err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}

	if acquired {
		s.initializeAsLeader(ctx)
	} else {
		s.initializeAsFollower()
	}

	return s.openIndexes()
}

// initializeAsLeader syncs repos, saves manifest, and unlocks.
func (s *Service) initializeAsLeader(ctx context.Context) {
	slog.Info("Acquired sync leader lock, starting sync")
	if err := s.SyncAll(ctx); err != nil {
		slog.Error("Sync failed", "error", err)
	}
	if err := s.saveManifest(); err != nil {
		slog.Error("Failed to save manifest", "error", err)
	}
	if err := s.lock.Unlock(); err != nil {
		slog.Error("Failed to unlock", "error", err)
	}
}

// initializeAsFollower waits for the leader to finish, then opens indexes.
func (s *Service) initializeAsFollower() {
	slog.Info("Another instance is syncing, waiting for completion")
	if err := s.lock.Lock(s.settings.SyncTimeout); err != nil {
		slog.Warn("Timeout waiting for sync, using existing indexes", "error", err)
	} else {
		if err := s.lock.Unlock(); err != nil {
			slog.Error("Failed to unlock", "error", err)
		}
	}
}

// SyncAll synchronizes all configured repositories.
func (s *Service) SyncAll(ctx context.Context) error {
	urls := s.settings.URLs
	if len(urls) == 0 {
		return nil
	}

	// Remove stale repos from manifest
	removed := s.manifest.RemoveStaleRepos(urls)
	for _, repoID := range removed {
		slog.Info("Removing stale repository", "repo_id", repoID)
		if err := s.indexer.DeleteIndex(repoID); err != nil {
			slog.Error("Failed to delete index for stale repo", "repo_id", repoID, "error", err)
		}
		// Clean up repo directory
		repoDir := filepath.Join(s.settings.BaseDir, "repos", repoID)
		if err := os.RemoveAll(repoDir); err != nil {
			slog.Error("Failed to remove stale repo directory", "repo_id", repoID, "error", err)
		}
	}

	// Use semaphore to limit parallel syncs
	sem := make(chan struct{}, MaxParallelSyncs)
	var wg sync.WaitGroup
	errChan := make(chan error, len(urls))

	for _, url := range urls {
		repoID := URLToRepoID(url)
		wg.Add(1)
		go func(url, repoID string) {
			defer wg.Done()
			sem <- struct{}{}        // Acquire
			defer func() { <-sem }() // Release

			if err := s.syncRepo(ctx, repoID, url); err != nil {
				slog.Error("Failed to sync repository", "repo_id", repoID, "error", err)
				s.manifest.SetRepoError(repoID, err.Error())
				errChan <- fmt.Errorf("sync %s: %w", repoID, err)
			} else {
				s.manifest.ClearRepoError(repoID)
			}
		}(url, repoID)
	}

	wg.Wait()
	close(errChan)

	// Collect errors
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	s.manifest.UpdateLastSync()

	if len(errs) > 0 {
		return fmt.Errorf("%d repository sync(s) failed", len(errs))
	}
	return nil
}

// syncRepo syncs a single repository.
func (s *Service) syncRepo(ctx context.Context, repoID, url string) error {
	repoDir := filepath.Join(s.settings.BaseDir, "repos", repoID)

	// Get current state
	state := s.manifest.GetRepoState(repoID)
	isNew := !s.manifest.HasRepo(repoID) || state.ClonedAt.IsZero()

	if isNew {
		// Clone new repository
		slog.Info("Cloning repository", "repo_id", repoID, "url", url)
		if err := s.git.Clone(ctx, url, repoDir); err != nil {
			return fmt.Errorf("clone failed: %w", err)
		}
		state.URL = url
		state.ClonedAt = time.Now()
	} else {
		// Fetch updates
		slog.Info("Fetching repository updates", "repo_id", repoID)
		if err := s.git.Fetch(ctx, repoDir); err != nil {
			return fmt.Errorf("fetch failed: %w", err)
		}
	}

	// Get current HEAD commit
	currentCommit, err := s.git.GetHeadCommit(ctx, repoDir)
	if err != nil {
		return fmt.Errorf("failed to get HEAD commit: %w", err)
	}

	// Check if reindex is needed
	needsReindex := isNew || state.LastIndexed == "" || currentCommit != state.LastCommit

	if needsReindex {
		if !isNew && state.LastIndexed != "" && currentCommit != state.LastCommit {
			// Reset to latest
			if err := s.git.Reset(ctx, repoDir); err != nil {
				return fmt.Errorf("reset failed: %w", err)
			}

			// Try incremental index if we have previous commit
			if state.LastCommit != "" {
				changedFiles, err := s.git.GetChangedFiles(ctx, repoDir, state.LastCommit, currentCommit)
				if err == nil && len(changedFiles) > 0 && len(changedFiles) <= 100 {
					slog.Info("Incremental indexing", "repo_id", repoID, "changed_files", len(changedFiles))
					indexed, err := s.indexer.IncrementalIndex(repoID, repoDir, changedFiles)
					if err != nil {
						slog.Warn("Incremental index failed, falling back to full index", "error", err)
					} else {
						state.LastCommit = currentCommit
						state.LastIndexed = currentCommit
						state.LastPull = time.Now()
						s.manifest.SetRepoState(repoID, *state)
						slog.Info("Incremental index complete", "repo_id", repoID, "indexed", indexed)
						return nil
					}
				} else if err == nil && len(changedFiles) > 100 {
					slog.Info("Too many changed files for incremental index, falling back to full index", "repo_id", repoID, "changed_files", len(changedFiles))
				}
			}
		}

		// Full reindex
		slog.Info("Full indexing", "repo_id", repoID)
		fileCount, err := s.indexer.FullIndex(repoID, repoDir)
		if err != nil {
			return fmt.Errorf("full index failed: %w", err)
		}

		state.LastCommit = currentCommit
		state.LastIndexed = currentCommit
		state.FileCount = fileCount
		state.LastPull = time.Now()
		s.manifest.SetRepoState(repoID, *state)
		slog.Info("Full index complete", "repo_id", repoID, "file_count", fileCount)
	} else {
		slog.Info("Repository already up to date", "repo_id", repoID)
	}

	return nil
}

// openIndexes opens all indexes and creates the alias.
func (s *Service) openIndexes() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Get all repo IDs that have indexes
	var indexedRepos []string
	for _, url := range s.settings.URLs {
		repoID := URLToRepoID(url)
		if s.indexer.IndexExists(repoID) {
			indexedRepos = append(indexedRepos, repoID)
		}
	}

	if len(indexedRepos) == 0 {
		slog.Warn("No indexes available")
		s.ready = false
		return nil
	}

	// Create alias combining all indexes
	alias, err := s.indexer.CreateAlias(indexedRepos)
	if err != nil {
		return fmt.Errorf("failed to create index alias: %w", err)
	}

	s.alias = alias
	s.ready = true
	slog.Info("Indexes ready", "count", len(indexedRepos))
	return nil
}

// saveManifest saves the manifest to disk.
func (s *Service) saveManifest() error {
	manifestPath := filepath.Join(s.settings.BaseDir, ManifestFilename)
	return s.manifest.Save(manifestPath)
}

// IsReady returns true if indexes are ready for search.
func (s *Service) IsReady() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ready
}

// GetIndexAlias returns the combined index for searching.
func (s *Service) GetIndexAlias() (bleve.IndexAlias, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready || s.alias == nil {
		return nil, fmt.Errorf("indexes not ready")
	}
	return s.alias, nil
}

// GetRepoDir returns the directory for a repository.
func (s *Service) GetRepoDir(repoID string) string {
	return filepath.Join(s.settings.BaseDir, "repos", repoID)
}

// MaxResults returns the configured maximum number of search results.
func (s *Service) MaxResults() int {
	return s.settings.MaxResults
}

// MaxFileSize returns the configured maximum file size for reading.
func (s *Service) MaxFileSize() int64 {
	return s.settings.MaxFileSize
}

// GetSettings returns the service settings.
func (s *Service) GetSettings() *config.GitReposSettings {
	return s.settings
}

// SetGitOperations allows injecting a custom GitOperations implementation for testing.
func (s *Service) SetGitOperations(ops GitOperations) {
	s.git = ops
}

// Close releases all resources.
func (s *Service) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.alias != nil {
		if err := s.alias.Close(); err != nil {
			return fmt.Errorf("failed to close alias: %w", err)
		}
		s.alias = nil
	}

	s.ready = false
	return nil
}
