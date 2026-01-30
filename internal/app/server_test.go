package app

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sha1n/mcp-relic-server/internal/config"
)

func TestNewSSEServer_NoAuth(t *testing.T) {
	impl := &mcp.Implementation{Name: "test", Version: "1.0"}
	server := mcp.NewServer(impl, nil)

	settings := &config.Settings{
		Host: "localhost",
		Port: 8080,
		Auth: config.AuthSettings{Type: config.AuthTypeNone},
	}

	srv, err := NewSSEServer(server, settings)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if srv == nil {
		t.Fatal("Expected server to be created")
	}
	if srv.Addr != "localhost:8080" {
		t.Errorf("Expected addr 'localhost:8080', got '%s'", srv.Addr)
	}
}

func TestNewSSEServer_BasicAuth(t *testing.T) {
	impl := &mcp.Implementation{Name: "test", Version: "1.0"}
	server := mcp.NewServer(impl, nil)

	settings := &config.Settings{
		Host: "localhost",
		Port: 9090,
		Auth: config.AuthSettings{
			Type: config.AuthTypeBasic,
			Basic: config.BasicAuthSettings{
				Username: "admin",
				Password: "secret",
			},
		},
	}

	srv, err := NewSSEServer(server, settings)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if srv == nil {
		t.Fatal("Expected server to be created")
	}
}

func TestNewSSEServer_APIKeyAuth(t *testing.T) {
	impl := &mcp.Implementation{Name: "test", Version: "1.0"}
	server := mcp.NewServer(impl, nil)

	settings := &config.Settings{
		Host: "localhost",
		Port: 9090,
		Auth: config.AuthSettings{
			Type:    config.AuthTypeAPIKey,
			APIKeys: []string{"key1", "key2"},
		},
	}

	srv, err := NewSSEServer(server, settings)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if srv == nil {
		t.Fatal("Expected server to be created")
	}
}

func TestNewSSEServer_InvalidAuth(t *testing.T) {
	impl := &mcp.Implementation{Name: "test", Version: "1.0"}
	server := mcp.NewServer(impl, nil)

	settings := &config.Settings{
		Host: "localhost",
		Port: 9090,
		Auth: config.AuthSettings{
			Type: config.AuthTypeBasic,
			// Missing username and password
		},
	}

	_, err := NewSSEServer(server, settings)
	if err == nil {
		t.Error("Expected error for invalid auth settings")
	}
}

func TestNewSSEServer_HealthEndpoint(t *testing.T) {
	impl := &mcp.Implementation{Name: "test", Version: "1.0"}
	server := mcp.NewServer(impl, nil)

	settings := &config.Settings{
		Host: "localhost",
		Port: 8080,
		Auth: config.AuthSettings{Type: config.AuthTypeNone},
	}

	srv, err := NewSSEServer(server, settings)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Test health endpoint
	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
	if rec.Body.String() != "ok" {
		t.Errorf("Expected body 'ok', got '%s'", rec.Body.String())
	}
	if rec.Header().Get("Content-Type") != "text/plain; charset=utf-8" {
		t.Errorf("Expected Content-Type 'text/plain; charset=utf-8', got '%s'", rec.Header().Get("Content-Type"))
	}
}

func TestNewSSEServer_HealthEndpointBypassesAuth(t *testing.T) {
	impl := &mcp.Implementation{Name: "test", Version: "1.0"}
	server := mcp.NewServer(impl, nil)

	settings := &config.Settings{
		Host: "localhost",
		Port: 8080,
		Auth: config.AuthSettings{
			Type: config.AuthTypeBasic,
			Basic: config.BasicAuthSettings{
				Username: "admin",
				Password: "secret",
			},
		},
	}

	srv, err := NewSSEServer(server, settings)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Test health endpoint without auth - should still work
	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200 for /health without auth, got %d", rec.Code)
	}
}

func TestNewSSEServer_SSEEndpointRequiresAuth(t *testing.T) {
	impl := &mcp.Implementation{Name: "test", Version: "1.0"}
	server := mcp.NewServer(impl, nil)

	settings := &config.Settings{
		Host: "localhost",
		Port: 8080,
		Auth: config.AuthSettings{
			Type: config.AuthTypeBasic,
			Basic: config.BasicAuthSettings{
				Username: "admin",
				Password: "secret",
			},
		},
	}

	srv, err := NewSSEServer(server, settings)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Test SSE endpoint without auth - should fail
	req := httptest.NewRequest("GET", "/sse", nil)
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 for /sse without auth, got %d", rec.Code)
	}
}
