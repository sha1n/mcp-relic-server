package gitrepos

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	// ManifestVersion is the current schema version
	ManifestVersion = 1

	// ManifestFilename is the default manifest filename
	ManifestFilename = "manifest.json"
)

// Manifest stores the sync state for all repositories.
type Manifest struct {
	Version  int                  `json:"version"`
	LastSync time.Time            `json:"last_sync"`
	Repos    map[string]RepoState `json:"repos"`
}

// RepoState stores the sync state for a single repository.
type RepoState struct {
	URL         string    `json:"url"`
	ClonedAt    time.Time `json:"cloned_at"`
	LastPull    time.Time `json:"last_pull"`
	LastCommit  string    `json:"last_commit"`
	LastIndexed string    `json:"last_indexed"`
	FileCount   int       `json:"file_count"`
	Error       string    `json:"error,omitempty"`
}

// NewManifest creates a new empty manifest.
func NewManifest() *Manifest {
	return &Manifest{
		Version: ManifestVersion,
		Repos:   make(map[string]RepoState),
	}
}

// LoadManifest reads a manifest from disk, or creates a new one if it doesn't exist.
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewManifest(), nil
		}
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	// Initialize repos map if nil (for backwards compatibility)
	if manifest.Repos == nil {
		manifest.Repos = make(map[string]RepoState)
	}

	return &manifest, nil
}

// Save writes the manifest to disk atomically.
// Uses write-to-temp + rename pattern to prevent corruption.
func (m *Manifest) Save(path string) error {
	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create manifest directory: %w", err)
	}

	// Marshal to JSON with indentation for readability
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	// Write to temporary file first
	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write manifest temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempPath, path); err != nil {
		// Clean up temp file on error
		_ = os.Remove(tempPath)
		return fmt.Errorf("failed to rename manifest file: %w", err)
	}

	return nil
}

// GetRepoState returns the state for a repository, creating it if it doesn't exist.
func (m *Manifest) GetRepoState(repoID string) *RepoState {
	state, ok := m.Repos[repoID]
	if !ok {
		state = RepoState{}
		m.Repos[repoID] = state
	}
	return &state
}

// SetRepoState updates the state for a repository.
func (m *Manifest) SetRepoState(repoID string, state RepoState) {
	m.Repos[repoID] = state
}

// HasRepo returns true if the repository exists in the manifest.
func (m *Manifest) HasRepo(repoID string) bool {
	_, ok := m.Repos[repoID]
	return ok
}

// RemoveRepo removes a repository from the manifest.
func (m *Manifest) RemoveRepo(repoID string) {
	delete(m.Repos, repoID)
}

// GetRepoIDs returns a list of all repository IDs in the manifest.
func (m *Manifest) GetRepoIDs() []string {
	ids := make([]string, 0, len(m.Repos))
	for id := range m.Repos {
		ids = append(ids, id)
	}
	return ids
}

// RemoveStaleRepos removes repositories not in the given URL list.
// Returns the list of removed repository IDs.
func (m *Manifest) RemoveStaleRepos(urls []string) []string {
	// Build set of expected repo IDs from URLs
	expected := make(map[string]bool)
	for _, url := range urls {
		repoID := URLToRepoID(url)
		expected[repoID] = true
	}

	// Find repos to remove
	var removed []string
	for repoID := range m.Repos {
		if !expected[repoID] {
			removed = append(removed, repoID)
		}
	}

	// Remove stale repos
	for _, repoID := range removed {
		delete(m.Repos, repoID)
	}

	return removed
}

// UpdateLastSync updates the last sync timestamp.
func (m *Manifest) UpdateLastSync() {
	m.LastSync = time.Now()
}

// NeedsSyncCheck returns true if enough time has passed since the last sync.
func (m *Manifest) NeedsSyncCheck(interval time.Duration) bool {
	if m.LastSync.IsZero() {
		return true
	}
	return time.Since(m.LastSync) >= interval
}

// GetReposWithErrors returns a list of repositories that have errors.
func (m *Manifest) GetReposWithErrors() map[string]string {
	result := make(map[string]string)
	for repoID, state := range m.Repos {
		if state.Error != "" {
			result[repoID] = state.Error
		}
	}
	return result
}

// ClearRepoError clears the error for a repository.
func (m *Manifest) ClearRepoError(repoID string) {
	if state, ok := m.Repos[repoID]; ok {
		state.Error = ""
		m.Repos[repoID] = state
	}
}

// SetRepoError sets the error for a repository.
func (m *Manifest) SetRepoError(repoID string, err string) {
	if state, ok := m.Repos[repoID]; ok {
		state.Error = err
		m.Repos[repoID] = state
	} else {
		m.Repos[repoID] = RepoState{Error: err}
	}
}
