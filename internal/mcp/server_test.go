package mcp

import (
	"os"
	"testing"

	"github.com/sha1n/mcp-relic-server/internal/config"
	"github.com/sha1n/mcp-relic-server/internal/gitrepos"
)

func TestCreateServer(t *testing.T) {
	cfg := ServerConfig{
		Name:    "test-server",
		Version: "1.0.0",
	}

	server := CreateServer(cfg)
	if server == nil {
		t.Fatal("Expected server to be created")
	}
}

func TestCreateServer_EmptyConfig(t *testing.T) {
	cfg := ServerConfig{}

	server := CreateServer(cfg)
	if server == nil {
		t.Fatal("Expected server to be created even with empty config")
	}
}

func TestCreateServer_WithVersion(t *testing.T) {
	cfg := ServerConfig{
		Name:    "relic-mcp",
		Version: "2.0.0",
	}

	server := CreateServer(cfg)
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

	server := CreateServer(cfg)
	if server == nil {
		t.Fatal("Expected server to be created without git repos service")
	}
}

func TestCreateServer_WithGitReposService(t *testing.T) {
	// Create a temporary directory for the test
	dir := t.TempDir()

	// Create a minimal git repos service
	settings := &config.GitReposSettings{
		Enabled:     true,
		BaseDir:     dir,
		MaxFileSize: 256 * 1024,
		MaxResults:  20,
	}

	svc, err := gitrepos.NewService(settings)
	if err != nil {
		t.Fatalf("Failed to create git repos service: %v", err)
	}
	defer func() {
		if err := svc.Close(); err != nil {
			t.Errorf("Failed to close service: %v", err)
		}
	}()

	cfg := ServerConfig{
		Name:        "test-server",
		Version:     "1.0.0",
		GitReposSvc: svc,
	}

	server := CreateServer(cfg)
	if server == nil {
		t.Fatal("Expected server to be created with git repos service")
	}
}

func TestCreateServer_ToolsRegistered(t *testing.T) {
	// Create a temporary directory for the test
	dir := t.TempDir()

	// Create repos and indexes directories
	if err := os.MkdirAll(dir+"/repos", 0755); err != nil {
		t.Fatalf("Failed to create repos dir: %v", err)
	}
	if err := os.MkdirAll(dir+"/indexes", 0755); err != nil {
		t.Fatalf("Failed to create indexes dir: %v", err)
	}

	// Create a git repos service
	settings := &config.GitReposSettings{
		Enabled:     true,
		BaseDir:     dir,
		MaxFileSize: 256 * 1024,
		MaxResults:  20,
	}

	svc, err := gitrepos.NewService(settings)
	if err != nil {
		t.Fatalf("Failed to create git repos service: %v", err)
	}
	defer func() {
		if err := svc.Close(); err != nil {
			t.Errorf("Failed to close service: %v", err)
		}
	}()

	cfg := ServerConfig{
		Name:        "test-server",
		Version:     "1.0.0",
		GitReposSvc: svc,
	}

	server := CreateServer(cfg)
	if server == nil {
		t.Fatal("Expected server to be created")
	}

	// The server is created with tools registered.
	// The MCP SDK doesn't expose a way to list registered tools,
	// so we just verify the server was created successfully.
	// Integration tests will verify tools are accessible via MCP protocol.
}
