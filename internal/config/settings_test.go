package config

import (
	"os"
	"strings"
	"testing"

	"github.com/spf13/pflag"
)

func TestLoadSettings_Defaults(t *testing.T) {
	_ = os.Unsetenv("RELIC_MCP_PORT")
	_ = os.Unsetenv("RELIC_MCP_AUTH_TYPE")

	settings, err := LoadSettings()
	if err != nil {
		t.Fatalf("Failed to load settings: %v", err)
	}

	if settings.Port != 8080 {
		t.Errorf("Expected default port 8080, got %d", settings.Port)
	}
	if settings.Auth.Type != AuthTypeNone {
		t.Errorf("Expected default auth type '%s', got '%s'", AuthTypeNone, settings.Auth.Type)
	}
	if settings.Transport != "stdio" {
		t.Errorf("Expected default transport 'stdio', got '%s'", settings.Transport)
	}
	if settings.Host != "0.0.0.0" {
		t.Errorf("Expected default host '0.0.0.0', got '%s'", settings.Host)
	}
}

func TestLoadSettings_EnvVars(t *testing.T) {
	t.Setenv("RELIC_MCP_PORT", "9090")
	t.Setenv("RELIC_MCP_AUTH_TYPE", "basic")
	t.Setenv("RELIC_MCP_AUTH_BASIC_USERNAME", "admin")

	settings, err := LoadSettings()
	if err != nil {
		t.Fatalf("Failed to load settings: %v", err)
	}

	if settings.Port != 9090 {
		t.Errorf("Expected port 9090, got %d", settings.Port)
	}
	if settings.Auth.Type != AuthTypeBasic {
		t.Errorf("Expected auth type '%s', got '%s'", AuthTypeBasic, settings.Auth.Type)
	}
	if settings.Auth.Basic.Username != "admin" {
		t.Errorf("Expected username 'admin', got '%s'", settings.Auth.Basic.Username)
	}
}

func TestLoadSettings_APIKeys_EnvVar(t *testing.T) {
	t.Setenv("RELIC_MCP_AUTH_API_KEYS", "key1, key2,key3")

	settings, err := LoadSettings()
	if err != nil {
		t.Fatalf("Failed to load settings: %v", err)
	}

	if len(settings.Auth.APIKeys) != 3 {
		t.Fatalf("Expected 3 API keys, got %d", len(settings.Auth.APIKeys))
	}
	if settings.Auth.APIKeys[0] != "key1" {
		t.Errorf("Expected key1, got '%s'", settings.Auth.APIKeys[0])
	}
	if settings.Auth.APIKeys[1] != "key2" {
		t.Errorf("Expected key2, got '%s'", settings.Auth.APIKeys[1])
	}
	if settings.Auth.APIKeys[2] != "key3" {
		t.Errorf("Expected key3, got '%s'", settings.Auth.APIKeys[2])
	}
}

func TestLoadSettings_APIKeys_SingleKey(t *testing.T) {
	t.Setenv("RELIC_MCP_AUTH_API_KEYS", "singlekey")

	settings, err := LoadSettings()
	if err != nil {
		t.Fatalf("Failed to load settings: %v", err)
	}
	if len(settings.Auth.APIKeys) != 1 {
		t.Fatalf("Expected 1 API key, got %d", len(settings.Auth.APIKeys))
	}
	if settings.Auth.APIKeys[0] != "singlekey" {
		t.Errorf("Expected singlekey, got '%s'", settings.Auth.APIKeys[0])
	}
}

func TestLoadSettings_EnvFile(t *testing.T) {
	content := []byte("host=127.0.0.2\nport=7000")
	tmpEnv := ".env"
	if err := os.WriteFile(tmpEnv, content, 0644); err != nil {
		t.Fatalf("Failed to create .env file: %v", err)
	}
	defer func() { _ = os.Remove(tmpEnv) }()

	settings, err := LoadSettings()
	if err != nil {
		t.Fatalf("Failed to load settings: %v", err)
	}

	if settings.Host != "127.0.0.2" {
		t.Errorf("Expected host 127.0.0.2, got %s", settings.Host)
	}
	if settings.Port != 7000 {
		t.Errorf("Expected port 7000, got %d", settings.Port)
	}
}

func TestLoadSettings_InvalidConfig(t *testing.T) {
	t.Setenv("RELIC_MCP_PORT", "not-a-number")

	_, err := LoadSettings()
	if err == nil {
		t.Fatal("Expected error for invalid port type")
	}
}

func TestLoadSettingsWithFlags_CLIOverridesEnv(t *testing.T) {
	t.Setenv("RELIC_MCP_PORT", "9090")
	t.Setenv("RELIC_MCP_TRANSPORT", "sse")

	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.Int("port", 0, "")
	flags.String("transport", "", "")
	_ = flags.Set("port", "7777")
	_ = flags.Set("transport", "stdio")

	settings, err := LoadSettingsWithFlags(flags)
	if err != nil {
		t.Fatalf("Failed to load settings: %v", err)
	}

	if settings.Port != 7777 {
		t.Errorf("Expected CLI port 7777, got %d", settings.Port)
	}
	if settings.Transport != "stdio" {
		t.Errorf("Expected CLI transport 'stdio', got '%s'", settings.Transport)
	}
}

func TestLoadSettingsWithFlags_EnvOverridesDefault(t *testing.T) {
	t.Setenv("RELIC_MCP_HOST", "192.168.1.1")

	settings, err := LoadSettingsWithFlags(nil)
	if err != nil {
		t.Fatalf("Failed to load settings: %v", err)
	}

	if settings.Host != "192.168.1.1" {
		t.Errorf("Expected env host '192.168.1.1', got '%s'", settings.Host)
	}
}

func TestLoadSettingsWithFlags_NilFlags(t *testing.T) {
	_ = os.Unsetenv("RELIC_MCP_PORT")

	settings, err := LoadSettingsWithFlags(nil)
	if err != nil {
		t.Fatalf("Failed to load settings: %v", err)
	}

	if settings.Port != 8080 {
		t.Errorf("Expected default port 8080, got %d", settings.Port)
	}
}

func TestLoadSettingsWithFlags_AllFlagTypes(t *testing.T) {
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.String("transport", "", "")
	flags.String("host", "", "")
	flags.Int("port", 0, "")
	flags.String("auth-type", "", "")
	flags.String("auth-basic-username", "", "")
	flags.String("auth-basic-password", "", "")
	flags.StringSlice("auth-api-keys", nil, "")

	_ = flags.Set("transport", "sse")
	_ = flags.Set("host", "localhost")
	_ = flags.Set("port", "3000")
	_ = flags.Set("auth-type", "basic")
	_ = flags.Set("auth-basic-username", "testuser")
	_ = flags.Set("auth-basic-password", "testpass")

	settings, err := LoadSettingsWithFlags(flags)
	if err != nil {
		t.Fatalf("Failed to load settings: %v", err)
	}

	if settings.Transport != "sse" {
		t.Errorf("Expected transport 'sse', got '%s'", settings.Transport)
	}
	if settings.Host != "localhost" {
		t.Errorf("Expected host 'localhost', got '%s'", settings.Host)
	}
	if settings.Port != 3000 {
		t.Errorf("Expected port 3000, got %d", settings.Port)
	}
	if settings.Auth.Type != "basic" {
		t.Errorf("Expected auth type 'basic', got '%s'", settings.Auth.Type)
	}
	if settings.Auth.Basic.Username != "testuser" {
		t.Errorf("Expected username 'testuser', got '%s'", settings.Auth.Basic.Username)
	}
	if settings.Auth.Basic.Password != "testpass" {
		t.Errorf("Expected password 'testpass', got '%s'", settings.Auth.Basic.Password)
	}
}

// --- ValidateSettings Tests ---

func TestValidateSettings_ValidNone(t *testing.T) {
	s := &Settings{Transport: "stdio", Auth: AuthSettings{Type: AuthTypeNone}}
	if err := ValidateSettings(s); err != nil {
		t.Errorf("Expected no error for valid none auth, got: %v", err)
	}
}

func TestValidateSettings_ValidNone_EmptyType(t *testing.T) {
	s := &Settings{Transport: "stdio", Auth: AuthSettings{Type: ""}}
	if err := ValidateSettings(s); err != nil {
		t.Errorf("Expected no error for empty auth type, got: %v", err)
	}
}

func TestValidateSettings_ValidBasic(t *testing.T) {
	s := &Settings{
		Transport: "stdio",
		Auth: AuthSettings{
			Type: AuthTypeBasic,
			Basic: BasicAuthSettings{
				Username: "admin",
				Password: "secret",
			},
		},
	}
	if err := ValidateSettings(s); err != nil {
		t.Errorf("Expected no error for valid basic auth, got: %v", err)
	}
}

func TestValidateSettings_ValidAPIKey(t *testing.T) {
	s := &Settings{
		Transport: "stdio",
		Auth: AuthSettings{
			Type:    AuthTypeAPIKey,
			APIKeys: []string{"key1", "key2"},
		},
	}
	if err := ValidateSettings(s); err != nil {
		t.Errorf("Expected no error for valid apikey auth, got: %v", err)
	}
}

func TestValidateSettings_NoneWithCredentials(t *testing.T) {
	tests := []struct {
		name     string
		settings Settings
	}{
		{
			name: "none with username",
			settings: Settings{
				Transport: "stdio",
				Auth: AuthSettings{
					Type:  AuthTypeNone,
					Basic: BasicAuthSettings{Username: "admin"},
				},
			},
		},
		{
			name: "none with password",
			settings: Settings{
				Transport: "stdio",
				Auth: AuthSettings{
					Type:  AuthTypeNone,
					Basic: BasicAuthSettings{Password: "secret"},
				},
			},
		},
		{
			name: "none with api keys",
			settings: Settings{
				Transport: "stdio",
				Auth: AuthSettings{
					Type:    AuthTypeNone,
					APIKeys: []string{"key1"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSettings(&tt.settings)
			if err == nil {
				t.Fatal("Expected error for none with credentials")
			}
			if !strings.Contains(err.Error(), "incompatible") {
				t.Errorf("Expected 'incompatible' in error, got: %v", err)
			}
		})
	}
}

func TestValidateSettings_BasicAuthMissingUsername(t *testing.T) {
	s := &Settings{
		Transport: "stdio",
		Auth: AuthSettings{
			Type: AuthTypeBasic,
			Basic: BasicAuthSettings{
				Password: "secret",
			},
		},
	}
	err := ValidateSettings(s)
	if err == nil {
		t.Fatal("Expected error for basic auth without username")
	}
	if !strings.Contains(err.Error(), "username and password") {
		t.Errorf("Expected 'username and password' in error, got: %v", err)
	}
}

func TestValidateSettings_BasicAuthMissingPassword(t *testing.T) {
	s := &Settings{
		Transport: "stdio",
		Auth: AuthSettings{
			Type: AuthTypeBasic,
			Basic: BasicAuthSettings{
				Username: "admin",
			},
		},
	}
	err := ValidateSettings(s)
	if err == nil {
		t.Fatal("Expected error for basic auth without password")
	}
}

func TestValidateSettings_BasicAuthWithAPIKeys(t *testing.T) {
	s := &Settings{
		Transport: "stdio",
		Auth: AuthSettings{
			Type: AuthTypeBasic,
			Basic: BasicAuthSettings{
				Username: "admin",
				Password: "secret",
			},
			APIKeys: []string{"key1"},
		},
	}
	err := ValidateSettings(s)
	if err == nil {
		t.Fatal("Expected error for basic + api keys")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("Expected 'mutually exclusive' in error, got: %v", err)
	}
}

func TestValidateSettings_APIKeyMissingKeys(t *testing.T) {
	s := &Settings{
		Transport: "stdio",
		Auth: AuthSettings{
			Type: AuthTypeAPIKey,
		},
	}
	err := ValidateSettings(s)
	if err == nil {
		t.Fatal("Expected error for apikey without keys")
	}
	if !strings.Contains(err.Error(), "requires at least one") {
		t.Errorf("Expected 'requires at least one' in error, got: %v", err)
	}
}

func TestValidateSettings_APIKeyWithBasicCreds(t *testing.T) {
	s := &Settings{
		Transport: "stdio",
		Auth: AuthSettings{
			Type:    AuthTypeAPIKey,
			APIKeys: []string{"key1"},
			Basic: BasicAuthSettings{
				Username: "admin",
			},
		},
	}
	err := ValidateSettings(s)
	if err == nil {
		t.Fatal("Expected error for apikey + basic creds")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("Expected 'mutually exclusive' in error, got: %v", err)
	}
}

func TestValidateSettings_UnknownAuthType(t *testing.T) {
	s := &Settings{
		Transport: "stdio",
		Auth: AuthSettings{
			Type: "oauth",
		},
	}
	err := ValidateSettings(s)
	if err == nil {
		t.Fatal("Expected error for unknown auth type")
	}
	if !strings.Contains(err.Error(), "unknown auth-type") {
		t.Errorf("Expected 'unknown auth-type' in error, got: %v", err)
	}
}

// --- Transport Validation Tests ---

func TestValidateSettings_ValidTransportStdio(t *testing.T) {
	s := &Settings{Transport: "stdio", Auth: AuthSettings{Type: AuthTypeNone}}
	if err := ValidateSettings(s); err != nil {
		t.Errorf("Expected no error for valid stdio transport, got: %v", err)
	}
}

func TestValidateSettings_ValidTransportSSE(t *testing.T) {
	s := &Settings{Transport: "sse", Auth: AuthSettings{Type: AuthTypeNone}}
	if err := ValidateSettings(s); err != nil {
		t.Errorf("Expected no error for valid sse transport, got: %v", err)
	}
}

func TestValidateSettings_InvalidTransport(t *testing.T) {
	tests := []struct {
		name      string
		transport string
	}{
		{"empty transport", ""},
		{"http transport", "http"},
		{"websocket transport", "websocket"},
		{"unknown transport", "foobar"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Settings{
				Transport: tt.transport,
				Auth:      AuthSettings{Type: AuthTypeNone},
			}
			err := ValidateSettings(s)
			if err == nil {
				t.Fatalf("Expected error for transport %q", tt.transport)
			}
			if !strings.Contains(err.Error(), "transport must be") {
				t.Errorf("Expected 'transport must be' in error, got: %v", err)
			}
		})
	}
}
