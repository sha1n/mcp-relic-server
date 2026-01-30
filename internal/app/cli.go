package app

import (
	"time"

	"github.com/spf13/pflag"
)

// RegisterFlags registers all CLI flags on the given FlagSet
func RegisterFlags(flags *pflag.FlagSet) {
	// Transport and server flags
	flags.StringP("transport", "t", "", "Transport type: stdio or sse")
	flags.StringP("host", "H", "", "Host for SSE transport")
	flags.IntP("port", "p", 0, "Port for SSE transport")

	// Auth flags
	flags.StringP("auth-type", "a", "", "Authentication type: none, basic, or apikey")
	flags.StringP("auth-basic-username", "u", "", "Basic auth username")
	flags.StringP("auth-basic-password", "P", "", "Basic auth password")
	flags.StringSliceP("auth-api-keys", "k", nil, "API keys (comma-separated)")

	// Git repos flags
	flags.Bool("git-repos-enabled", false, "Enable git repository indexing")
	flags.StringSlice("git-repos-urls", nil, "Git repository SSH URLs (comma-separated)")
	flags.String("git-repos-base-dir", "", "Base directory for git data (default: ~/.relic-mcp)")
	flags.Duration("git-repos-sync-interval", 15*time.Minute, "Minimum interval between syncs")
	flags.Duration("git-repos-sync-timeout", 60*time.Second, "Maximum time to wait for sync lock")
	flags.Int64("git-repos-max-file-size", 256*1024, "Skip files larger than this (bytes)")
	flags.Int("git-repos-max-results", 20, "Maximum search results")
}
