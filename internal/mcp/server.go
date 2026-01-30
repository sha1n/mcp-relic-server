package mcp

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ServerConfig contains configuration for creating an MCP server
type ServerConfig struct {
	Name    string
	Version string
}

// CreateServer creates and configures the MCP server
func CreateServer(cfg ServerConfig) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{
		Name:    cfg.Name,
		Version: cfg.Version,
	}, nil)

	// Tools will be registered here in future implementations
	// RegisterSearchTool(s, ...)
	// RegisterReadTool(s, ...)

	return s
}
