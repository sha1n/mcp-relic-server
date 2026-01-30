package config

import (
	"errors"
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
	Enabled      bool          `mapstructure:"enabled"`
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
	v.SetDefault("git_repos.enabled", false)
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
	_ = v.BindEnv("auth.type", "RELIC_MCP_AUTH_TYPE")
	_ = v.BindEnv("auth.basic.username", "RELIC_MCP_AUTH_BASIC_USERNAME")
	_ = v.BindEnv("auth.basic.password", "RELIC_MCP_AUTH_BASIC_PASSWORD")
	_ = v.BindEnv("auth.api_keys", "RELIC_MCP_AUTH_API_KEYS")

	// Git repos env var bindings
	_ = v.BindEnv("git_repos.enabled", "RELIC_MCP_GIT_REPOS_ENABLED")
	_ = v.BindEnv("git_repos.urls", "RELIC_MCP_GIT_REPOS_URLS")
	_ = v.BindEnv("git_repos.base_dir", "RELIC_MCP_GIT_REPOS_BASE_DIR")
	_ = v.BindEnv("git_repos.sync_interval", "RELIC_MCP_GIT_REPOS_SYNC_INTERVAL")
	_ = v.BindEnv("git_repos.sync_timeout", "RELIC_MCP_GIT_REPOS_SYNC_TIMEOUT")
	_ = v.BindEnv("git_repos.max_file_size", "RELIC_MCP_GIT_REPOS_MAX_FILE_SIZE")
	_ = v.BindEnv("git_repos.max_results", "RELIC_MCP_GIT_REPOS_MAX_RESULTS")

	// Bind CLI flags if provided (highest priority)
	if flags != nil {
		_ = v.BindPFlag("transport", flags.Lookup("transport"))
		_ = v.BindPFlag("host", flags.Lookup("host"))
		_ = v.BindPFlag("port", flags.Lookup("port"))
		_ = v.BindPFlag("auth.type", flags.Lookup("auth-type"))
		_ = v.BindPFlag("auth.basic.username", flags.Lookup("auth-basic-username"))
		_ = v.BindPFlag("auth.basic.password", flags.Lookup("auth-basic-password"))
		_ = v.BindPFlag("auth.api_keys", flags.Lookup("auth-api-keys"))

		// Git repos CLI flags
		_ = v.BindPFlag("git_repos.enabled", flags.Lookup("git-repos-enabled"))
		_ = v.BindPFlag("git_repos.urls", flags.Lookup("git-repos-urls"))
		_ = v.BindPFlag("git_repos.base_dir", flags.Lookup("git-repos-base-dir"))
		_ = v.BindPFlag("git_repos.sync_interval", flags.Lookup("git-repos-sync-interval"))
		_ = v.BindPFlag("git_repos.sync_timeout", flags.Lookup("git-repos-sync-timeout"))
		_ = v.BindPFlag("git_repos.max_file_size", flags.Lookup("git-repos-max-file-size"))
		_ = v.BindPFlag("git_repos.max_results", flags.Lookup("git-repos-max-results"))
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
	if !g.Enabled {
		return nil // No validation needed when disabled
	}

	if len(g.URLs) == 0 {
		return errors.New("git-repos-enabled requires at least one repository URL (git-repos-urls)")
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
