package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

// --- GitReposSettings Tests ---

func TestLoadSettings_GitReposDefaults(t *testing.T) {
	// Clear any existing env vars
	_ = os.Unsetenv("RELIC_MCP_GIT_REPOS_ENABLED")
	_ = os.Unsetenv("RELIC_MCP_GIT_REPOS_URLS")
	_ = os.Unsetenv("RELIC_MCP_GIT_REPOS_BASE_DIR")
	_ = os.Unsetenv("RELIC_MCP_GIT_REPOS_SYNC_INTERVAL")
	_ = os.Unsetenv("RELIC_MCP_GIT_REPOS_SYNC_TIMEOUT")
	_ = os.Unsetenv("RELIC_MCP_GIT_REPOS_MAX_FILE_SIZE")
	_ = os.Unsetenv("RELIC_MCP_GIT_REPOS_MAX_RESULTS")

	settings, err := LoadSettings()
	if err != nil {
		t.Fatalf("Failed to load settings: %v", err)
	}

	if settings.GitRepos.Enabled {
		t.Error("Expected git repos disabled by default")
	}

	if len(settings.GitRepos.URLs) != 0 {
		t.Errorf("Expected empty URLs by default, got %d", len(settings.GitRepos.URLs))
	}

	// Check default base dir contains .relic-mcp
	if !strings.HasSuffix(settings.GitRepos.BaseDir, ".relic-mcp") {
		t.Errorf("Expected base dir to end with '.relic-mcp', got '%s'", settings.GitRepos.BaseDir)
	}

	if settings.GitRepos.SyncInterval != 15*time.Minute {
		t.Errorf("Expected sync interval 15m, got %v", settings.GitRepos.SyncInterval)
	}

	if settings.GitRepos.SyncTimeout != 60*time.Second {
		t.Errorf("Expected sync timeout 60s, got %v", settings.GitRepos.SyncTimeout)
	}

	if settings.GitRepos.MaxFileSize != 256*1024 {
		t.Errorf("Expected max file size 256KB, got %d", settings.GitRepos.MaxFileSize)
	}

	if settings.GitRepos.MaxResults != 20 {
		t.Errorf("Expected max results 20, got %d", settings.GitRepos.MaxResults)
	}
}

func TestLoadSettings_GitReposEnvVars(t *testing.T) {
	t.Setenv("RELIC_MCP_GIT_REPOS_ENABLED", "true")
	t.Setenv("RELIC_MCP_GIT_REPOS_URLS", "git@github.com:org/repo1.git,git@github.com:org/repo2.git")
	t.Setenv("RELIC_MCP_GIT_REPOS_BASE_DIR", "/custom/path")
	t.Setenv("RELIC_MCP_GIT_REPOS_SYNC_INTERVAL", "30m")
	t.Setenv("RELIC_MCP_GIT_REPOS_SYNC_TIMEOUT", "120s")
	t.Setenv("RELIC_MCP_GIT_REPOS_MAX_FILE_SIZE", "512000")
	t.Setenv("RELIC_MCP_GIT_REPOS_MAX_RESULTS", "50")

	settings, err := LoadSettings()
	if err != nil {
		t.Fatalf("Failed to load settings: %v", err)
	}

	if !settings.GitRepos.Enabled {
		t.Error("Expected git repos enabled")
	}

	if len(settings.GitRepos.URLs) != 2 {
		t.Fatalf("Expected 2 URLs, got %d", len(settings.GitRepos.URLs))
	}
	if settings.GitRepos.URLs[0] != "git@github.com:org/repo1.git" {
		t.Errorf("Expected first URL 'git@github.com:org/repo1.git', got '%s'", settings.GitRepos.URLs[0])
	}
	if settings.GitRepos.URLs[1] != "git@github.com:org/repo2.git" {
		t.Errorf("Expected second URL 'git@github.com:org/repo2.git', got '%s'", settings.GitRepos.URLs[1])
	}

	if settings.GitRepos.BaseDir != "/custom/path" {
		t.Errorf("Expected base dir '/custom/path', got '%s'", settings.GitRepos.BaseDir)
	}

	if settings.GitRepos.SyncInterval != 30*time.Minute {
		t.Errorf("Expected sync interval 30m, got %v", settings.GitRepos.SyncInterval)
	}

	if settings.GitRepos.SyncTimeout != 120*time.Second {
		t.Errorf("Expected sync timeout 120s, got %v", settings.GitRepos.SyncTimeout)
	}

	if settings.GitRepos.MaxFileSize != 512000 {
		t.Errorf("Expected max file size 512000, got %d", settings.GitRepos.MaxFileSize)
	}

	if settings.GitRepos.MaxResults != 50 {
		t.Errorf("Expected max results 50, got %d", settings.GitRepos.MaxResults)
	}
}

func TestLoadSettings_GitReposURLsTrimSpaces(t *testing.T) {
	t.Setenv("RELIC_MCP_GIT_REPOS_URLS", " git@github.com:org/repo1.git , git@github.com:org/repo2.git ")

	settings, err := LoadSettings()
	if err != nil {
		t.Fatalf("Failed to load settings: %v", err)
	}

	if len(settings.GitRepos.URLs) != 2 {
		t.Fatalf("Expected 2 URLs, got %d", len(settings.GitRepos.URLs))
	}
	if settings.GitRepos.URLs[0] != "git@github.com:org/repo1.git" {
		t.Errorf("Expected trimmed URL, got '%s'", settings.GitRepos.URLs[0])
	}
	if settings.GitRepos.URLs[1] != "git@github.com:org/repo2.git" {
		t.Errorf("Expected trimmed URL, got '%s'", settings.GitRepos.URLs[1])
	}
}

func TestLoadSettings_GitReposURLsFilterEmpty(t *testing.T) {
	t.Setenv("RELIC_MCP_GIT_REPOS_URLS", "git@github.com:org/repo1.git,,git@github.com:org/repo2.git,")

	settings, err := LoadSettings()
	if err != nil {
		t.Fatalf("Failed to load settings: %v", err)
	}

	if len(settings.GitRepos.URLs) != 2 {
		t.Fatalf("Expected 2 URLs (empty filtered out), got %d: %v", len(settings.GitRepos.URLs), settings.GitRepos.URLs)
	}
}

func TestLoadSettings_GitReposBaseDirExpandHome(t *testing.T) {
	t.Setenv("RELIC_MCP_GIT_REPOS_BASE_DIR", "~/custom-relic")

	settings, err := LoadSettings()
	if err != nil {
		t.Fatalf("Failed to load settings: %v", err)
	}

	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, "custom-relic")
	if settings.GitRepos.BaseDir != expected {
		t.Errorf("Expected base dir '%s', got '%s'", expected, settings.GitRepos.BaseDir)
	}
}

func TestLoadSettingsWithFlags_GitReposFlags(t *testing.T) {
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.Bool("git-repos-enabled", false, "")
	flags.StringSlice("git-repos-urls", nil, "")
	flags.String("git-repos-base-dir", "", "")
	flags.Duration("git-repos-sync-interval", 0, "")
	flags.Duration("git-repos-sync-timeout", 0, "")
	flags.Int64("git-repos-max-file-size", 0, "")
	flags.Int("git-repos-max-results", 0, "")

	_ = flags.Set("git-repos-enabled", "true")
	_ = flags.Set("git-repos-urls", "git@github.com:org/repo.git")
	_ = flags.Set("git-repos-base-dir", "/flag/path")
	_ = flags.Set("git-repos-sync-interval", "5m")
	_ = flags.Set("git-repos-sync-timeout", "30s")
	_ = flags.Set("git-repos-max-file-size", "1024")
	_ = flags.Set("git-repos-max-results", "10")

	settings, err := LoadSettingsWithFlags(flags)
	if err != nil {
		t.Fatalf("Failed to load settings: %v", err)
	}

	if !settings.GitRepos.Enabled {
		t.Error("Expected git repos enabled from flag")
	}

	if len(settings.GitRepos.URLs) != 1 || settings.GitRepos.URLs[0] != "git@github.com:org/repo.git" {
		t.Errorf("Expected URL from flag, got %v", settings.GitRepos.URLs)
	}

	if settings.GitRepos.BaseDir != "/flag/path" {
		t.Errorf("Expected base dir '/flag/path', got '%s'", settings.GitRepos.BaseDir)
	}

	if settings.GitRepos.SyncInterval != 5*time.Minute {
		t.Errorf("Expected sync interval 5m, got %v", settings.GitRepos.SyncInterval)
	}

	if settings.GitRepos.SyncTimeout != 30*time.Second {
		t.Errorf("Expected sync timeout 30s, got %v", settings.GitRepos.SyncTimeout)
	}

	if settings.GitRepos.MaxFileSize != 1024 {
		t.Errorf("Expected max file size 1024, got %d", settings.GitRepos.MaxFileSize)
	}

	if settings.GitRepos.MaxResults != 10 {
		t.Errorf("Expected max results 10, got %d", settings.GitRepos.MaxResults)
	}
}

func TestLoadSettingsWithFlags_GitReposFlagsOverrideEnv(t *testing.T) {
	t.Setenv("RELIC_MCP_GIT_REPOS_ENABLED", "false")
	t.Setenv("RELIC_MCP_GIT_REPOS_MAX_RESULTS", "100")

	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.Bool("git-repos-enabled", false, "")
	flags.Int("git-repos-max-results", 0, "")

	_ = flags.Set("git-repos-enabled", "true")
	_ = flags.Set("git-repos-max-results", "25")

	settings, err := LoadSettingsWithFlags(flags)
	if err != nil {
		t.Fatalf("Failed to load settings: %v", err)
	}

	if !settings.GitRepos.Enabled {
		t.Error("Expected flag to override env for enabled")
	}

	if settings.GitRepos.MaxResults != 25 {
		t.Errorf("Expected flag to override env for max results, got %d", settings.GitRepos.MaxResults)
	}
}

// --- GitRepos Validation Tests ---

func TestValidateSettings_GitReposDisabled(t *testing.T) {
	s := &Settings{
		Transport: "stdio",
		Auth:      AuthSettings{Type: AuthTypeNone},
		GitRepos:  GitReposSettings{Enabled: false},
	}
	if err := ValidateSettings(s); err != nil {
		t.Errorf("Expected no error for disabled git repos, got: %v", err)
	}
}

func TestValidateSettings_GitReposValid(t *testing.T) {
	s := &Settings{
		Transport: "stdio",
		Auth:      AuthSettings{Type: AuthTypeNone},
		GitRepos: GitReposSettings{
			Enabled:      true,
			URLs:         []string{"git@github.com:org/repo.git"},
			BaseDir:      "/tmp/test",
			SyncInterval: 15 * time.Minute,
			SyncTimeout:  60 * time.Second,
			MaxFileSize:  256 * 1024,
			MaxResults:   20,
		},
	}
	if err := ValidateSettings(s); err != nil {
		t.Errorf("Expected no error for valid git repos config, got: %v", err)
	}
}

func TestValidateSettings_GitReposEnabledNoURLs(t *testing.T) {
	s := &Settings{
		Transport: "stdio",
		Auth:      AuthSettings{Type: AuthTypeNone},
		GitRepos: GitReposSettings{
			Enabled:      true,
			URLs:         []string{},
			BaseDir:      "/tmp/test",
			SyncInterval: 15 * time.Minute,
			SyncTimeout:  60 * time.Second,
			MaxFileSize:  256 * 1024,
			MaxResults:   20,
		},
	}
	err := ValidateSettings(s)
	if err == nil {
		t.Fatal("Expected error for enabled git repos without URLs")
	}
	if !strings.Contains(err.Error(), "requires at least one repository URL") {
		t.Errorf("Expected 'requires at least one repository URL' in error, got: %v", err)
	}
}

func TestValidateSettings_GitReposInvalidSyncInterval(t *testing.T) {
	s := &Settings{
		Transport: "stdio",
		Auth:      AuthSettings{Type: AuthTypeNone},
		GitRepos: GitReposSettings{
			Enabled:      true,
			URLs:         []string{"git@github.com:org/repo.git"},
			BaseDir:      "/tmp/test",
			SyncInterval: 0,
			SyncTimeout:  60 * time.Second,
			MaxFileSize:  256 * 1024,
			MaxResults:   20,
		},
	}
	err := ValidateSettings(s)
	if err == nil {
		t.Fatal("Expected error for zero sync interval")
	}
	if !strings.Contains(err.Error(), "sync-interval must be positive") {
		t.Errorf("Expected 'sync-interval must be positive' in error, got: %v", err)
	}
}

func TestValidateSettings_GitReposInvalidSyncTimeout(t *testing.T) {
	s := &Settings{
		Transport: "stdio",
		Auth:      AuthSettings{Type: AuthTypeNone},
		GitRepos: GitReposSettings{
			Enabled:      true,
			URLs:         []string{"git@github.com:org/repo.git"},
			BaseDir:      "/tmp/test",
			SyncInterval: 15 * time.Minute,
			SyncTimeout:  0,
			MaxFileSize:  256 * 1024,
			MaxResults:   20,
		},
	}
	err := ValidateSettings(s)
	if err == nil {
		t.Fatal("Expected error for zero sync timeout")
	}
	if !strings.Contains(err.Error(), "sync-timeout must be positive") {
		t.Errorf("Expected 'sync-timeout must be positive' in error, got: %v", err)
	}
}

func TestValidateSettings_GitReposInvalidMaxFileSize(t *testing.T) {
	s := &Settings{
		Transport: "stdio",
		Auth:      AuthSettings{Type: AuthTypeNone},
		GitRepos: GitReposSettings{
			Enabled:      true,
			URLs:         []string{"git@github.com:org/repo.git"},
			BaseDir:      "/tmp/test",
			SyncInterval: 15 * time.Minute,
			SyncTimeout:  60 * time.Second,
			MaxFileSize:  0,
			MaxResults:   20,
		},
	}
	err := ValidateSettings(s)
	if err == nil {
		t.Fatal("Expected error for zero max file size")
	}
	if !strings.Contains(err.Error(), "max-file-size must be positive") {
		t.Errorf("Expected 'max-file-size must be positive' in error, got: %v", err)
	}
}

func TestValidateSettings_GitReposInvalidMaxResults(t *testing.T) {
	s := &Settings{
		Transport: "stdio",
		Auth:      AuthSettings{Type: AuthTypeNone},
		GitRepos: GitReposSettings{
			Enabled:      true,
			URLs:         []string{"git@github.com:org/repo.git"},
			BaseDir:      "/tmp/test",
			SyncInterval: 15 * time.Minute,
			SyncTimeout:  60 * time.Second,
			MaxFileSize:  256 * 1024,
			MaxResults:   0,
		},
	}
	err := ValidateSettings(s)
	if err == nil {
		t.Fatal("Expected error for zero max results")
	}
	if !strings.Contains(err.Error(), "max-results must be positive") {
		t.Errorf("Expected 'max-results must be positive' in error, got: %v", err)
	}
}

func TestValidateSettings_GitReposEmptyBaseDir(t *testing.T) {
	s := &Settings{
		Transport: "stdio",
		Auth:      AuthSettings{Type: AuthTypeNone},
		GitRepos: GitReposSettings{
			Enabled:      true,
			URLs:         []string{"git@github.com:org/repo.git"},
			BaseDir:      "",
			SyncInterval: 15 * time.Minute,
			SyncTimeout:  60 * time.Second,
			MaxFileSize:  256 * 1024,
			MaxResults:   20,
		},
	}
	err := ValidateSettings(s)
	if err == nil {
		t.Fatal("Expected error for empty base dir")
	}
	if !strings.Contains(err.Error(), "base-dir cannot be empty") {
		t.Errorf("Expected 'base-dir cannot be empty' in error, got: %v", err)
	}
}

// --- Helper Function Tests ---

func TestExpandHomeDir(t *testing.T) {
	home, _ := os.UserHomeDir()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"tilde prefix", "~/test", filepath.Join(home, "test")},
		{"tilde only", "~", home},
		{"no tilde", "/absolute/path", "/absolute/path"},
		{"tilde in middle", "/path/~/test", "/path/~/test"},
		{"relative path", "relative/path", "relative/path"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandHomeDir(tt.input)
			if result != tt.expected {
				t.Errorf("expandHomeDir(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFilterEmptyStrings(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{"no empties", []string{"a", "b", "c"}, []string{"a", "b", "c"}},
		{"with empties", []string{"a", "", "b", "", "c"}, []string{"a", "b", "c"}},
		{"all empties", []string{"", "", ""}, nil},
		{"nil input", nil, nil},
		{"single empty", []string{""}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterEmptyStrings(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("filterEmptyStrings(%v) = %v, want %v", tt.input, result, tt.expected)
				return
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("filterEmptyStrings(%v) = %v, want %v", tt.input, result, tt.expected)
					break
				}
			}
		})
	}
}
