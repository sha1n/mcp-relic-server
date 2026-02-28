package gitrepos

import (
	"context"
	"time"

	"github.com/blevesearch/bleve/v2"
)

// SearchService defines what the search handler needs from the service layer.
type SearchService interface {
	IsReady() bool
	GetIndexAlias() (bleve.IndexAlias, error)
	MaxResults() int
}

// ReadService defines what the read handler needs from the service layer.
type ReadService interface {
	IsReady() bool
	GetRepoDir(repoID string) string
	MaxFileSize() int64
}

// GitOperations abstracts git client operations for testing.
type GitOperations interface {
	Clone(ctx context.Context, url, destDir string) error
	Fetch(ctx context.Context, repoDir string) error
	Reset(ctx context.Context, repoDir string) error
	GetHeadCommit(ctx context.Context, repoDir string) (string, error)
	GetChangedFiles(ctx context.Context, repoDir, fromCommit, toCommit string) ([]string, error)
}

// IndexOperations abstracts indexing operations for testing.
type IndexOperations interface {
	FullIndex(repoID, repoDir string) (int, error)
	IncrementalIndex(repoID, repoDir string, changedFiles []string) (int, error)
	DeleteIndex(repoID string) error
	IndexExists(repoID string) bool
	CreateAlias(repoIDs []string) (bleve.IndexAlias, error)
}

// ManifestOperations abstracts manifest operations for testing.
type ManifestOperations interface {
	GetRepoState(repoID string) RepoState
	SetRepoState(repoID string, state RepoState)
	HasRepo(repoID string) bool
	RemoveStaleRepos(urls []string) []string
	UpdateLastSync()
	ClearRepoError(repoID string)
	SetRepoError(repoID string, err string)
	Save(path string) error
}

// SyncLock abstracts file locking for testing.
type SyncLock interface {
	TryLock() (bool, error)
	Lock(timeout time.Duration) error
	Unlock() error
}
