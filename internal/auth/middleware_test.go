package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sha1n/mcp-relic-server/internal/config"
)

func TestNewMiddleware_None(t *testing.T) {
	settings := config.AuthSettings{Type: config.AuthTypeNone}
	middleware, err := NewMiddleware(settings)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
}

func TestNewMiddleware_EmptyType(t *testing.T) {
	settings := config.AuthSettings{Type: ""}
	middleware, err := NewMiddleware(settings)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
}

func TestNewMiddleware_BasicAuth_Valid(t *testing.T) {
	settings := config.AuthSettings{
		Type: config.AuthTypeBasic,
		Basic: config.BasicAuthSettings{
			Username: "admin",
			Password: "secret",
		},
	}
	middleware, err := NewMiddleware(settings)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.SetBasicAuth("admin", "secret")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
}

func TestNewMiddleware_BasicAuth_Invalid(t *testing.T) {
	settings := config.AuthSettings{
		Type: config.AuthTypeBasic,
		Basic: config.BasicAuthSettings{
			Username: "admin",
			Password: "secret",
		},
	}
	middleware, err := NewMiddleware(settings)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.SetBasicAuth("admin", "wrongpassword")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rec.Code)
	}
}

func TestNewMiddleware_BasicAuth_NoCredentials(t *testing.T) {
	settings := config.AuthSettings{
		Type: config.AuthTypeBasic,
		Basic: config.BasicAuthSettings{
			Username: "admin",
			Password: "secret",
		},
	}
	middleware, err := NewMiddleware(settings)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rec.Code)
	}
	if rec.Header().Get("WWW-Authenticate") == "" {
		t.Error("Expected WWW-Authenticate header")
	}
}

func TestNewMiddleware_BasicAuth_MissingCredentials(t *testing.T) {
	tests := []struct {
		name     string
		settings config.AuthSettings
	}{
		{
			name: "missing username",
			settings: config.AuthSettings{
				Type: config.AuthTypeBasic,
				Basic: config.BasicAuthSettings{
					Password: "secret",
				},
			},
		},
		{
			name: "missing password",
			settings: config.AuthSettings{
				Type: config.AuthTypeBasic,
				Basic: config.BasicAuthSettings{
					Username: "admin",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewMiddleware(tt.settings)
			if err == nil {
				t.Error("Expected error for missing credentials")
			}
		})
	}
}

func TestNewMiddleware_APIKey_Valid(t *testing.T) {
	settings := config.AuthSettings{
		Type:    config.AuthTypeAPIKey,
		APIKeys: []string{"key1", "key2"},
	}
	middleware, err := NewMiddleware(settings)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "key2")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
}

func TestNewMiddleware_APIKey_Invalid(t *testing.T) {
	settings := config.AuthSettings{
		Type:    config.AuthTypeAPIKey,
		APIKeys: []string{"key1", "key2"},
	}
	middleware, err := NewMiddleware(settings)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "wrongkey")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rec.Code)
	}
}

func TestNewMiddleware_APIKey_Missing(t *testing.T) {
	settings := config.AuthSettings{
		Type:    config.AuthTypeAPIKey,
		APIKeys: []string{"key1"},
	}
	middleware, err := NewMiddleware(settings)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rec.Code)
	}
}

func TestNewMiddleware_APIKey_NoKeys(t *testing.T) {
	settings := config.AuthSettings{
		Type:    config.AuthTypeAPIKey,
		APIKeys: []string{},
	}
	_, err := NewMiddleware(settings)
	if err == nil {
		t.Error("Expected error for no API keys")
	}
}

func TestNewMiddleware_UnknownType(t *testing.T) {
	settings := config.AuthSettings{Type: "oauth"}
	_, err := NewMiddleware(settings)
	if err == nil {
		t.Error("Expected error for unknown auth type")
	}
}

func TestExcludedPath_Health(t *testing.T) {
	settings := config.AuthSettings{
		Type: config.AuthTypeBasic,
		Basic: config.BasicAuthSettings{
			Username: "admin",
			Password: "secret",
		},
	}
	middleware, err := NewMiddleware(settings)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// /health should bypass auth
	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200 for /health, got %d", rec.Code)
	}
}

func TestIsExcludedPath(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"/health", true},
		{"/test", false},
		{"/api/health", false},
		{"/", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := isExcludedPath(tt.path); got != tt.expected {
				t.Errorf("isExcludedPath(%q) = %v, want %v", tt.path, got, tt.expected)
			}
		})
	}
}
