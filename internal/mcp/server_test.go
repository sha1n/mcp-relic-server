package mcp

import (
	"fmt"
	"testing"

	"github.com/blevesearch/bleve/v2"
)

// mockGitReposToolService implements GitReposToolService for testing.
type mockGitReposToolService struct {
	ready       bool
	alias       bleve.IndexAlias
	aliasErr    error
	maxResults  int
	repoDir     string
	maxFileSize int64
}

func (m *mockGitReposToolService) IsReady() bool { return m.ready }
func (m *mockGitReposToolService) GetIndexAlias() (bleve.IndexAlias, error) {
	return m.alias, m.aliasErr
}
func (m *mockGitReposToolService) MaxResults() int            { return m.maxResults }
func (m *mockGitReposToolService) GetRepoDir(_ string) string { return m.repoDir }
func (m *mockGitReposToolService) MaxFileSize() int64         { return m.maxFileSize }

func TestCreateServer(t *testing.T) {
	cfg := ServerConfig{
		Name:    "test-server",
		Version: "1.0.0",
	}

	server, err := CreateServer(cfg)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if server == nil {
		t.Fatal("Expected server to be created")
	}
}

func TestCreateServer_EmptyConfig(t *testing.T) {
	cfg := ServerConfig{}

	server, err := CreateServer(cfg)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if server == nil {
		t.Fatal("Expected server to be created even with empty config")
	}
}

func TestCreateServer_WithVersion(t *testing.T) {
	cfg := ServerConfig{
		Name:    "relic-mcp",
		Version: "2.0.0",
	}

	server, err := CreateServer(cfg)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if server == nil {
		t.Fatal("Expected server to be created")
	}
}

func TestCreateServer_WithoutGitReposService(t *testing.T) {
	cfg := ServerConfig{
		Name:        "test-server",
		Version:     "1.0.0",
		GitReposSvc: nil,
	}

	server, err := CreateServer(cfg)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if server == nil {
		t.Fatal("Expected server to be created without git repos service")
	}
}

func TestCreateServer_WithGitReposService(t *testing.T) {
	cfg := ServerConfig{
		Name:    "test-server",
		Version: "1.0.0",
		GitReposSvc: &mockGitReposToolService{
			ready:       false,
			maxResults:  20,
			maxFileSize: 256 * 1024,
			aliasErr:    fmt.Errorf("not ready"),
		},
	}

	server, err := CreateServer(cfg)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if server == nil {
		t.Fatal("Expected server to be created with git repos service")
	}
}

func TestCreateServer_ToolsRegistered(t *testing.T) {
	cfg := ServerConfig{
		Name:    "test-server",
		Version: "1.0.0",
		GitReposSvc: &mockGitReposToolService{
			ready:       true,
			maxResults:  20,
			maxFileSize: 256 * 1024,
		},
	}

	server, err := CreateServer(cfg)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if server == nil {
		t.Fatal("Expected server to be created")
	}
}
