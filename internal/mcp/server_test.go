package mcp

import (
	"testing"
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
