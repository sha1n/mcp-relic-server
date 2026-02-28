package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Auth type constants
const (
	AuthTypeNone   = "none"
	AuthTypeBasic  = "basic"
	AuthTypeAPIKey = "apikey"
)

// AuthSettings configuration for authentication
type AuthSettings struct {
	Type    string            `mapstructure:"type"` // AuthTypeNone, AuthTypeBasic, or AuthTypeAPIKey
	Basic   BasicAuthSettings `mapstructure:"basic"`
	APIKeys []string          `mapstructure:"api_keys"`
}

// BasicAuthSettings configuration for basic auth
type BasicAuthSettings struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

// GitReposSettings configuration for git repository indexing
type GitReposSettings struct {
	URLs         []string      `mapstructure:"urls"`
	BaseDir      string        `mapstructure:"base_dir"`
	SyncInterval time.Duration `mapstructure:"sync_interval"`
	SyncTimeout  time.Duration `mapstructure:"sync_timeout"`
	MaxFileSize  int64         `mapstructure:"max_file_size"`
	MaxResults   int           `mapstructure:"max_results"`
}

// Settings application settings
type Settings struct {
	Transport string           `mapstructure:"transport"`
	Host      string           `mapstructure:"host"`
	Port      int              `mapstructure:"port"`
	Auth      AuthSettings     `mapstructure:"auth"`
	GitRepos  GitReposSettings `mapstructure:"git_repos"`
}

// LoadSettings loads settings from environment variables and optional .env file
func LoadSettings() (*Settings, error) {
	return LoadSettingsWithFlags(nil)
}

// LoadSettingsWithFlags loads settings with optional CLI flag overrides.
// Priority: CLI flags > environment variables > .env file > defaults.
// If flags is nil, only env vars and defaults are used.
func LoadSettingsWithFlags(flags *pflag.FlagSet) (*Settings, error) {
	v := viper.New()

	// Default values
	v.SetDefault("transport", "stdio")
	v.SetDefault("host", "0.0.0.0")
	v.SetDefault("port", 8080)
	v.SetDefault("auth.type", AuthTypeNone)

	// Git repos defaults
	v.SetDefault("git_repos.base_dir", defaultGitReposBaseDir())
	v.SetDefault("git_repos.sync_interval", 15*time.Minute)
	v.SetDefault("git_repos.sync_timeout", 60*time.Second)
	v.SetDefault("git_repos.max_file_size", int64(256*1024)) // 256KB
	v.SetDefault("git_repos.max_results", 20)

	// Environment variables
	v.SetEnvPrefix("RELIC_MCP")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Bind specific env vars for nested config
	for _, binding := range []struct {
		key    string
		envVar string
	}{
		{"auth.type", "RELIC_MCP_AUTH_TYPE"},
		{"auth.basic.username", "RELIC_MCP_AUTH_BASIC_USERNAME"},
		{"auth.basic.password", "RELIC_MCP_AUTH_BASIC_PASSWORD"},
		{"auth.api_keys", "RELIC_MCP_AUTH_API_KEYS"},
		{"git_repos.urls", "RELIC_MCP_GIT_REPOS_URLS"},
		{"git_repos.base_dir", "RELIC_MCP_GIT_REPOS_BASE_DIR"},
		{"git_repos.sync_interval", "RELIC_MCP_GIT_REPOS_SYNC_INTERVAL"},
		{"git_repos.sync_timeout", "RELIC_MCP_GIT_REPOS_SYNC_TIMEOUT"},
		{"git_repos.max_file_size", "RELIC_MCP_GIT_REPOS_MAX_FILE_SIZE"},
		{"git_repos.max_results", "RELIC_MCP_GIT_REPOS_MAX_RESULTS"},
	} {
		if err := v.BindEnv(binding.key, binding.envVar); err != nil {
			return nil, fmt.Errorf("failed to bind env var %s: %w", binding.envVar, err)
		}
	}

	// Bind CLI flags if provided (highest priority)
	if flags != nil {
		for _, binding := range []struct {
			key  string
			flag string
		}{
			{"transport", "transport"},
			{"host", "host"},
			{"port", "port"},
			{"auth.type", "auth-type"},
			{"auth.basic.username", "auth-basic-username"},
			{"auth.basic.password", "auth-basic-password"},
			{"auth.api_keys", "auth-api-keys"},
			{"git_repos.urls", "git-repos-urls"},
			{"git_repos.base_dir", "git-repos-base-dir"},
			{"git_repos.sync_interval", "git-repos-sync-interval"},
			{"git_repos.sync_timeout", "git-repos-sync-timeout"},
			{"git_repos.max_file_size", "git-repos-max-file-size"},
			{"git_repos.max_results", "git-repos-max-results"},
		} {
			if f := flags.Lookup(binding.flag); f != nil {
				if err := v.BindPFlag(binding.key, f); err != nil {
					return nil, fmt.Errorf("failed to bind flag %s: %w", binding.flag, err)
				}
			}
		}
	}

	// Helper to look for .env file
	v.SetConfigName(".env")
	v.SetConfigType("env")
	v.AddConfigPath(".")
	_ = v.ReadInConfig() // Ignore error if .env doesn't exist

	var settings Settings
	if err := v.Unmarshal(&settings); err != nil {
		return nil, err
	}

	// Handle explicit parsing of API keys if provided via env var as comma-separated string
	apiKeysEnv := os.Getenv("RELIC_MCP_AUTH_API_KEYS")
	if apiKeysEnv != "" {
		if len(settings.Auth.APIKeys) == 0 || (len(settings.Auth.APIKeys) == 1 && strings.Contains(settings.Auth.APIKeys[0], ",")) {
			settings.Auth.APIKeys = strings.Split(apiKeysEnv, ",")
		}
	}

	// Trim spaces from API keys
	for i := range settings.Auth.APIKeys {
		settings.Auth.APIKeys[i] = strings.TrimSpace(settings.Auth.APIKeys[i])
	}

	// Handle explicit parsing of git repos URLs if provided via env var as comma-separated string
	gitReposURLsEnv := os.Getenv("RELIC_MCP_GIT_REPOS_URLS")
	if gitReposURLsEnv != "" {
		if len(settings.GitRepos.URLs) == 0 || (len(settings.GitRepos.URLs) == 1 && strings.Contains(settings.GitRepos.URLs[0], ",")) {
			settings.GitRepos.URLs = strings.Split(gitReposURLsEnv, ",")
		}
	}

	// Trim spaces from git repos URLs
	for i := range settings.GitRepos.URLs {
		settings.GitRepos.URLs[i] = strings.TrimSpace(settings.GitRepos.URLs[i])
	}

	// Filter out empty URLs
	settings.GitRepos.URLs = filterEmptyStrings(settings.GitRepos.URLs)

	// Expand home directory in base_dir
	settings.GitRepos.BaseDir = expandHomeDir(settings.GitRepos.BaseDir)

	return &settings, nil
}

// defaultGitReposBaseDir returns the default base directory for git repos
func defaultGitReposBaseDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".relic-mcp"
	}
	return filepath.Join(home, ".relic-mcp")
}

// expandHomeDir expands ~ to the user's home directory
func expandHomeDir(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	if path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return home
	}
	return path
}

// filterEmptyStrings removes empty strings from a slice
func filterEmptyStrings(s []string) []string {
	var result []string
	for _, str := range s {
		if str != "" {
			result = append(result, str)
		}
	}
	return result
}

// ValidateSettings checks for conflicting configurations.
// Returns an error if the settings contain mutually exclusive or incomplete auth config.
func ValidateSettings(s *Settings) error {
	// Trim auth credentials before validation
	s.Auth.Basic.Username = strings.TrimSpace(s.Auth.Basic.Username)
	s.Auth.Basic.Password = strings.TrimSpace(s.Auth.Basic.Password)
	for i := range s.Auth.APIKeys {
		s.Auth.APIKeys[i] = strings.TrimSpace(s.Auth.APIKeys[i])
	}
	s.Auth.APIKeys = filterEmptyStrings(s.Auth.APIKeys)

	// Validate transport type
	switch s.Transport {
	case "stdio", "sse":
		// valid
	default:
		return errors.New("transport must be 'stdio' or 'sse', got: " + s.Transport)
	}

	hasBasicCreds := s.Auth.Basic.Username != "" || s.Auth.Basic.Password != ""
	hasAPIKeys := len(s.Auth.APIKeys) > 0

	switch s.Auth.Type {
	case AuthTypeNone, "":
		if hasBasicCreds || hasAPIKeys {
			return errors.New("auth-type 'none' is incompatible with auth credentials")
		}
	case AuthTypeBasic:
		if hasAPIKeys {
			return errors.New("auth-type 'basic' is mutually exclusive with auth-api-keys")
		}
		if s.Auth.Basic.Username == "" || s.Auth.Basic.Password == "" {
			return errors.New("auth-type 'basic' requires both username and password")
		}
	case AuthTypeAPIKey:
		if hasBasicCreds {
			return errors.New("auth-type 'apikey' is mutually exclusive with basic auth credentials")
		}
		if !hasAPIKeys {
			return errors.New("auth-type 'apikey' requires at least one API key")
		}
	default:
		return errors.New("unknown auth-type: " + s.Auth.Type)
	}

	// Validate git repos settings
	if err := validateGitReposSettings(&s.GitRepos); err != nil {
		return err
	}

	return nil
}

// validateGitReposSettings validates the git repos configuration
func validateGitReposSettings(g *GitReposSettings) error {
	if len(g.URLs) == 0 {
		return errors.New("at least one repository URL is required (git-repos-urls)")
	}

	if g.SyncInterval <= 0 {
		return errors.New("git-repos-sync-interval must be positive")
	}

	if g.SyncTimeout <= 0 {
		return errors.New("git-repos-sync-timeout must be positive")
	}

	if g.MaxFileSize <= 0 {
		return errors.New("git-repos-max-file-size must be positive")
	}

	if g.MaxResults <= 0 {
		return errors.New("git-repos-max-results must be positive")
	}

	if g.BaseDir == "" {
		return errors.New("git-repos-base-dir cannot be empty")
	}

	return nil
}
