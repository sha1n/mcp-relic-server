package gitrepos

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestNewManifest(t *testing.T) {
	m := NewManifest()

	if m.Version != ManifestVersion {
		t.Errorf("Version = %d, want %d", m.Version, ManifestVersion)
	}
	if m.Repos == nil {
		t.Error("Repos should be initialized")
	}
	if len(m.Repos) != 0 {
		t.Errorf("Repos should be empty, got %d entries", len(m.Repos))
	}
}

func TestLoadManifest_NewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")

	m, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}

	if m.Version != ManifestVersion {
		t.Errorf("Version = %d, want %d", m.Version, ManifestVersion)
	}
	if len(m.Repos) != 0 {
		t.Error("Expected empty repos for new manifest")
	}
}

func TestLoadManifest_ExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")

	// Create a manifest file
	original := &Manifest{
		Version:  1,
		LastSync: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
		Repos: map[string]RepoState{
			"github.com_org_repo": {
				URL:         "git@github.com:org/repo.git",
				ClonedAt:    time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC),
				LastPull:    time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
				LastCommit:  "abc123",
				LastIndexed: "abc123",
				FileCount:   100,
			},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	// Load and verify
	m, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}

	if m.Version != 1 {
		t.Errorf("Version = %d, want 1", m.Version)
	}
	if len(m.Repos) != 1 {
		t.Fatalf("Expected 1 repo, got %d", len(m.Repos))
	}

	state, ok := m.Repos["github.com_org_repo"]
	if !ok {
		t.Fatal("Expected github.com_org_repo in repos")
	}
	if state.URL != "git@github.com:org/repo.git" {
		t.Errorf("URL = %q", state.URL)
	}
	if state.FileCount != 100 {
		t.Errorf("FileCount = %d, want 100", state.FileCount)
	}
}

func TestLoadManifest_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")

	if err := os.WriteFile(path, []byte("not valid json"), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	_, err := LoadManifest(path)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestLoadManifest_NilRepos(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")

	// Write JSON with null repos
	if err := os.WriteFile(path, []byte(`{"version": 1, "repos": null}`), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	m, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}

	if m.Repos == nil {
		t.Error("Repos should be initialized even if null in JSON")
	}
}

func TestManifest_Save(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state", "manifest.json")

	m := NewManifest()
	m.LastSync = time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	m.Repos["github.com_org_repo"] = RepoState{
		URL:         "git@github.com:org/repo.git",
		LastCommit:  "def456",
		LastIndexed: "def456",
		FileCount:   50,
	}

	if err := m.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("File should exist: %v", err)
	}

	// Verify content
	loaded, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}

	if loaded.Version != ManifestVersion {
		t.Errorf("Version = %d, want %d", loaded.Version, ManifestVersion)
	}
	if len(loaded.Repos) != 1 {
		t.Errorf("Expected 1 repo, got %d", len(loaded.Repos))
	}

	state := loaded.Repos["github.com_org_repo"]
	if state.FileCount != 50 {
		t.Errorf("FileCount = %d, want 50", state.FileCount)
	}
}

func TestManifest_Save_CreatesDirectories(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "dirs", "manifest.json")

	m := NewManifest()
	if err := m.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Error("File should exist after save")
	}
}

func TestManifest_Save_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")

	// Save initial manifest
	m := NewManifest()
	m.Repos["repo1"] = RepoState{URL: "url1"}
	if err := m.Save(path); err != nil {
		t.Fatalf("Initial save failed: %v", err)
	}

	// Save updated manifest
	m.Repos["repo2"] = RepoState{URL: "url2"}
	if err := m.Save(path); err != nil {
		t.Fatalf("Updated save failed: %v", err)
	}

	// Verify temp file is cleaned up
	tempPath := path + ".tmp"
	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		t.Error("Temp file should be removed after successful save")
	}

	// Verify both repos are present
	loaded, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}
	if len(loaded.Repos) != 2 {
		t.Errorf("Expected 2 repos, got %d", len(loaded.Repos))
	}
}

func TestManifest_Save_Error(t *testing.T) {
	// Create a directory where we can't write
	dir := t.TempDir()
	readOnlyDir := filepath.Join(dir, "readonly")
	if err := os.Mkdir(readOnlyDir, 0555); err != nil {
		t.Fatalf("Failed to create read-only dir: %v", err)
	}

	manifest := NewManifest()
	// Try to save to readonly/state/manifest.json
	manifestPath := filepath.Join(readOnlyDir, "state", "manifest.json")

	err := manifest.Save(manifestPath)
	if err == nil {
		// If running as root (e.g. in some docker containers), permissions might be ignored.
		// But in standard environment this should fail.
		if os.Getuid() != 0 {
			t.Error("Expected error when saving to read-only directory")
		}
	}
}

func TestManifest_GetRepoState_Existing(t *testing.T) {
	m := NewManifest()
	m.Repos["repo1"] = RepoState{URL: "url1", FileCount: 10}

	state := m.GetRepoState("repo1")
	if state.URL != "url1" {
		t.Errorf("URL = %q, want 'url1'", state.URL)
	}
	if state.FileCount != 10 {
		t.Errorf("FileCount = %d, want 10", state.FileCount)
	}
}

func TestManifest_GetRepoState_New(t *testing.T) {
	m := NewManifest()

	state := m.GetRepoState("new_repo")
	if state.URL != "" {
		t.Error("Expected empty state for new repo")
	}

	// Verify it was added to the map
	if !m.HasRepo("new_repo") {
		t.Error("New repo should be added to manifest")
	}
}

func TestManifest_SetRepoState(t *testing.T) {
	m := NewManifest()

	state := RepoState{
		URL:         "git@github.com:org/repo.git",
		LastCommit:  "abc123",
		LastIndexed: "abc123",
		FileCount:   100,
	}
	m.SetRepoState("repo1", state)

	got := m.Repos["repo1"]
	if got.URL != state.URL {
		t.Errorf("URL = %q, want %q", got.URL, state.URL)
	}
	if got.FileCount != state.FileCount {
		t.Errorf("FileCount = %d, want %d", got.FileCount, state.FileCount)
	}
}

func TestManifest_HasRepo(t *testing.T) {
	m := NewManifest()
	m.Repos["repo1"] = RepoState{}

	if !m.HasRepo("repo1") {
		t.Error("HasRepo should return true for existing repo")
	}
	if m.HasRepo("repo2") {
		t.Error("HasRepo should return false for non-existing repo")
	}
}

func TestManifest_RemoveRepo(t *testing.T) {
	m := NewManifest()
	m.Repos["repo1"] = RepoState{}
	m.Repos["repo2"] = RepoState{}

	m.RemoveRepo("repo1")

	if m.HasRepo("repo1") {
		t.Error("repo1 should be removed")
	}
	if !m.HasRepo("repo2") {
		t.Error("repo2 should still exist")
	}
}

func TestManifest_GetRepoIDs(t *testing.T) {
	m := NewManifest()
	m.Repos["repo1"] = RepoState{}
	m.Repos["repo2"] = RepoState{}
	m.Repos["repo3"] = RepoState{}

	ids := m.GetRepoIDs()

	if len(ids) != 3 {
		t.Fatalf("Expected 3 IDs, got %d", len(ids))
	}

	for _, expected := range []string{"repo1", "repo2", "repo3"} {
		if !slices.Contains(ids, expected) {
			t.Errorf("Missing ID: %s", expected)
		}
	}
}

func TestManifest_RemoveStaleRepos(t *testing.T) {
	m := NewManifest()
	m.Repos["github.com_org_repo1"] = RepoState{URL: "git@github.com:org/repo1.git"}
	m.Repos["github.com_org_repo2"] = RepoState{URL: "git@github.com:org/repo2.git"}
	m.Repos["github.com_org_repo3"] = RepoState{URL: "git@github.com:org/repo3.git"}

	// Keep only repo1 and repo3
	urls := []string{
		"git@github.com:org/repo1.git",
		"git@github.com:org/repo3.git",
	}

	removed := m.RemoveStaleRepos(urls)

	if len(removed) != 1 {
		t.Fatalf("Expected 1 removed, got %d: %v", len(removed), removed)
	}
	if removed[0] != "github.com_org_repo2" {
		t.Errorf("Expected repo2 to be removed, got %s", removed[0])
	}

	if !m.HasRepo("github.com_org_repo1") {
		t.Error("repo1 should still exist")
	}
	if m.HasRepo("github.com_org_repo2") {
		t.Error("repo2 should be removed")
	}
	if !m.HasRepo("github.com_org_repo3") {
		t.Error("repo3 should still exist")
	}
}

func TestManifest_RemoveStaleRepos_AllRemoved(t *testing.T) {
	m := NewManifest()
	m.Repos["repo1"] = RepoState{}
	m.Repos["repo2"] = RepoState{}

	removed := m.RemoveStaleRepos([]string{})

	if len(removed) != 2 {
		t.Errorf("Expected 2 removed, got %d", len(removed))
	}
	if len(m.Repos) != 0 {
		t.Error("All repos should be removed")
	}
}

func TestManifest_RemoveStaleRepos_NoneRemoved(t *testing.T) {
	m := NewManifest()
	m.Repos["github.com_org_repo"] = RepoState{URL: "git@github.com:org/repo.git"}

	removed := m.RemoveStaleRepos([]string{"git@github.com:org/repo.git"})

	if len(removed) != 0 {
		t.Errorf("Expected 0 removed, got %d", len(removed))
	}
	if len(m.Repos) != 1 {
		t.Error("Repo should still exist")
	}
}

func TestManifest_UpdateLastSync(t *testing.T) {
	m := NewManifest()

	before := time.Now()
	m.UpdateLastSync()
	after := time.Now()

	if m.LastSync.Before(before) || m.LastSync.After(after) {
		t.Error("LastSync should be between before and after")
	}
}

func TestManifest_NeedsSyncCheck(t *testing.T) {
	m := NewManifest()
	interval := 15 * time.Minute

	// Zero time should need sync
	if !m.NeedsSyncCheck(interval) {
		t.Error("Zero time should need sync check")
	}

	// Recent sync should not need check
	m.LastSync = time.Now().Add(-5 * time.Minute)
	if m.NeedsSyncCheck(interval) {
		t.Error("Recent sync should not need check")
	}

	// Old sync should need check
	m.LastSync = time.Now().Add(-20 * time.Minute)
	if !m.NeedsSyncCheck(interval) {
		t.Error("Old sync should need check")
	}
}

func TestManifest_GetReposWithErrors(t *testing.T) {
	m := NewManifest()
	m.Repos["repo1"] = RepoState{Error: "clone failed"}
	m.Repos["repo2"] = RepoState{} // no error
	m.Repos["repo3"] = RepoState{Error: "auth error"}

	errors := m.GetReposWithErrors()

	if len(errors) != 2 {
		t.Fatalf("Expected 2 errors, got %d", len(errors))
	}
	if errors["repo1"] != "clone failed" {
		t.Errorf("repo1 error = %q", errors["repo1"])
	}
	if errors["repo3"] != "auth error" {
		t.Errorf("repo3 error = %q", errors["repo3"])
	}
	if _, ok := errors["repo2"]; ok {
		t.Error("repo2 should not be in errors")
	}
}

func TestManifest_ClearRepoError(t *testing.T) {
	m := NewManifest()
	m.Repos["repo1"] = RepoState{Error: "some error", FileCount: 10}

	m.ClearRepoError("repo1")

	state := m.Repos["repo1"]
	if state.Error != "" {
		t.Error("Error should be cleared")
	}
	if state.FileCount != 10 {
		t.Error("Other fields should be preserved")
	}
}

func TestManifest_ClearRepoError_NonExistent(t *testing.T) {
	m := NewManifest()

	// Should not panic
	m.ClearRepoError("nonexistent")

	if m.HasRepo("nonexistent") {
		t.Error("Should not create repo when clearing error")
	}
}

func TestManifest_SetRepoError(t *testing.T) {
	m := NewManifest()
	m.Repos["repo1"] = RepoState{FileCount: 10}

	m.SetRepoError("repo1", "sync failed")

	state := m.Repos["repo1"]
	if state.Error != "sync failed" {
		t.Errorf("Error = %q, want 'sync failed'", state.Error)
	}
	if state.FileCount != 10 {
		t.Error("Other fields should be preserved")
	}
}

func TestManifest_SetRepoError_NewRepo(t *testing.T) {
	m := NewManifest()

	m.SetRepoError("newrepo", "error message")

	if !m.HasRepo("newrepo") {
		t.Fatal("Should create repo when setting error")
	}
	state := m.Repos["newrepo"]
	if state.Error != "error message" {
		t.Errorf("Error = %q", state.Error)
	}
}

func TestRepoState_JSONRoundTrip(t *testing.T) {
	original := RepoState{
		URL:         "git@github.com:org/repo.git",
		ClonedAt:    time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC),
		LastPull:    time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
		LastCommit:  "abc123",
		LastIndexed: "def456",
		FileCount:   100,
		Error:       "some error",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded RepoState
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.URL != original.URL {
		t.Errorf("URL mismatch")
	}
	if !decoded.ClonedAt.Equal(original.ClonedAt) {
		t.Errorf("ClonedAt mismatch")
	}
	if decoded.LastCommit != original.LastCommit {
		t.Errorf("LastCommit mismatch")
	}
	if decoded.FileCount != original.FileCount {
		t.Errorf("FileCount mismatch")
	}
	if decoded.Error != original.Error {
		t.Errorf("Error mismatch")
	}
}

func TestRepoState_EmptyErrorOmitted(t *testing.T) {
	state := RepoState{
		URL:       "git@github.com:org/repo.git",
		FileCount: 10,
		// Error is empty
	}

	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	jsonStr := string(data)
	if strings.Contains(jsonStr, "error") {
		t.Error("Empty error should be omitted from JSON")
	}
}
