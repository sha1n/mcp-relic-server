package gitrepos

import (
	"context"
	"time"

	"github.com/blevesearch/bleve/v2"
)

// mockSearchService implements SearchService for handler tests.
type mockSearchService struct {
	ready      bool
	alias      bleve.IndexAlias
	aliasErr   error
	maxResults int
}

func (m *mockSearchService) IsReady() bool                            { return m.ready }
func (m *mockSearchService) GetIndexAlias() (bleve.IndexAlias, error) { return m.alias, m.aliasErr }
func (m *mockSearchService) MaxResults() int                          { return m.maxResults }

// mockReadService implements ReadService for handler tests.
type mockReadService struct {
	ready       bool
	repoDir     string
	maxFileSize int64
}

func (m *mockReadService) IsReady() bool              { return m.ready }
func (m *mockReadService) GetRepoDir(_ string) string { return m.repoDir }
func (m *mockReadService) MaxFileSize() int64         { return m.maxFileSize }

// mockGitOps implements GitOperations for service tests.
type mockGitOps struct {
	cloneErr        error
	fetchErr        error
	resetErr        error
	headCommit      string
	headCommitErr   error
	changedFiles    []string
	changedFilesErr error
}

func (m *mockGitOps) Clone(_ context.Context, _, _ string) error { return m.cloneErr }
func (m *mockGitOps) Fetch(_ context.Context, _ string) error    { return m.fetchErr }
func (m *mockGitOps) Reset(_ context.Context, _ string) error    { return m.resetErr }
func (m *mockGitOps) GetHeadCommit(_ context.Context, _ string) (string, error) {
	return m.headCommit, m.headCommitErr
}
func (m *mockGitOps) GetChangedFiles(_ context.Context, _, _, _ string) ([]string, error) {
	return m.changedFiles, m.changedFilesErr
}

// mockIndexOps implements IndexOperations for service tests.
type mockIndexOps struct {
	fullIndexCount int
	fullIndexErr   error
	incrIndexCount int
	incrIndexErr   error
	deleteErr      error
	existsMap      map[string]bool
	alias          bleve.IndexAlias
	aliasErr       error
}

func (m *mockIndexOps) FullIndex(_, _ string) (int, error) {
	return m.fullIndexCount, m.fullIndexErr
}
func (m *mockIndexOps) IncrementalIndex(_, _ string, _ []string) (int, error) {
	return m.incrIndexCount, m.incrIndexErr
}
func (m *mockIndexOps) DeleteIndex(_ string) error { return m.deleteErr }
func (m *mockIndexOps) IndexExists(repoID string) bool {
	if m.existsMap == nil {
		return false
	}
	return m.existsMap[repoID]
}
func (m *mockIndexOps) CreateAlias(_ []string) (bleve.IndexAlias, error) {
	return m.alias, m.aliasErr
}

// mockManifestOps implements ManifestOperations for service tests.
type mockManifestOps struct {
	repos       map[string]RepoState
	staleResult []string
	saveErr     error
}

func newMockManifestOps() *mockManifestOps {
	return &mockManifestOps{repos: make(map[string]RepoState)}
}

func (m *mockManifestOps) GetRepoState(repoID string) RepoState {
	return m.repos[repoID]
}
func (m *mockManifestOps) SetRepoState(repoID string, state RepoState) { m.repos[repoID] = state }
func (m *mockManifestOps) HasRepo(repoID string) bool {
	_, ok := m.repos[repoID]
	return ok
}
func (m *mockManifestOps) RemoveStaleRepos(_ []string) []string { return m.staleResult }
func (m *mockManifestOps) UpdateLastSync()                      {}
func (m *mockManifestOps) ClearRepoError(repoID string) {
	if state, ok := m.repos[repoID]; ok {
		state.Error = ""
		m.repos[repoID] = state
	}
}
func (m *mockManifestOps) SetRepoError(repoID string, err string) {
	if state, ok := m.repos[repoID]; ok {
		state.Error = err
		m.repos[repoID] = state
	} else {
		m.repos[repoID] = RepoState{Error: err}
	}
}
func (m *mockManifestOps) Save(_ string) error { return m.saveErr }

// mockSyncLock implements SyncLock for service tests.
type mockSyncLock struct {
	tryLockResult bool
	tryLockErr    error
	lockErr       error
	unlockErr     error
}

func (m *mockSyncLock) TryLock() (bool, error)     { return m.tryLockResult, m.tryLockErr }
func (m *mockSyncLock) Lock(_ time.Duration) error { return m.lockErr }
func (m *mockSyncLock) Unlock() error              { return m.unlockErr }
