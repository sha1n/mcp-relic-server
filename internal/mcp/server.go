package mcp

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sha1n/mcp-relic-server/internal/gitrepos"
)

// GitReposToolService combines what both search and read tools need.
type GitReposToolService interface {
	gitrepos.SearchService
	gitrepos.ReadService
}

// ServerConfig contains configuration for creating an MCP server
type ServerConfig struct {
	Name        string
	Version     string
	GitReposSvc GitReposToolService // Optional, nil if disabled
}

// CreateServer creates and configures the MCP server
func CreateServer(cfg ServerConfig) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{
		Name:    cfg.Name,
		Version: cfg.Version,
	}, nil)

	// Register git repos tools if service is provided
	if cfg.GitReposSvc != nil {
		gitrepos.RegisterSearchTool(s, cfg.GitReposSvc)
		gitrepos.RegisterReadTool(s, cfg.GitReposSvc)
	}

	return s
}
