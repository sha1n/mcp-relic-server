package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sha1n/mcp-relic-server/internal/config"
	"github.com/sha1n/mcp-relic-server/internal/gitrepos"
	mcputil "github.com/sha1n/mcp-relic-server/internal/mcp"
	"github.com/spf13/pflag"
)

// RunParams contains dependencies for the run function
type RunParams struct {
	LoadSettings      func(*pflag.FlagSet) (*config.Settings, error)
	ValidSettings     func(*config.Settings) error
	StartSSEServer    func(context.Context, *mcp.Server, *config.Settings) error
	CreateServer      func(context.Context, *config.Settings) (*mcp.Server, func(), error)
	CustomIOTransport mcp.Transport // Optional: for testing with custom IO
}

// DefaultRunParams returns production dependencies
func DefaultRunParams() RunParams {
	return RunParams{
		LoadSettings:   config.LoadSettingsWithFlags,
		ValidSettings:  config.ValidateSettings,
		StartSSEServer: StartSSEServer,
		CreateServer:   CreateMCPServer,
	}
}

// RunWithDeps executes the server with the provided dependencies
func RunWithDeps(ctx context.Context, params RunParams, flags *pflag.FlagSet, version string) error {
	// Load settings
	settings, err := params.LoadSettings(flags)
	if err != nil {
		return fmt.Errorf("failed to load settings: %w", err)
	}

	// Validate settings for conflicting configurations
	if err := params.ValidSettings(settings); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Configure logging - always use stderr to avoid buffering issues
	handler := slog.NewTextHandler(os.Stderr, nil)
	slog.SetDefault(slog.New(handler))

	slog.Info("Starting MCP RELIC server", "version", version)
	config.Log(settings)

	mcpServer, cleanup, err := params.CreateServer(ctx, settings)
	if err != nil {
		return err
	}
	if cleanup != nil {
		defer cleanup()
	}

	// Start server
	if settings.Transport == "stdio" {
		// Use custom transport if provided (for testing), otherwise use stdio
		transport := params.CustomIOTransport
		if transport == nil {
			transport = &mcp.StdioTransport{}
		}
		return mcpServer.Run(ctx, transport)
	} else {
		slog.Info("Starting SSE server", "host", settings.Host, "port", settings.Port)
		return params.StartSSEServer(ctx, mcpServer, settings)
	}
}

// CreateMCPServer creates the MCP server with registered tools
func CreateMCPServer(ctx context.Context, settings *config.Settings) (*mcp.Server, func(), error) {
	var gitReposSvc mcputil.GitReposToolService
	var cleanup func()

	svc, err := gitrepos.NewService(&settings.GitRepos)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create git repos service: %w", err)
	}

	// Set cleanup before Initialize so a panic during init doesn't leak the service
	cleanup = func() {
		if err := svc.Close(); err != nil {
			slog.Error("Failed to close git repos service", "error", err)
		}
	}

	if err := svc.Initialize(ctx); err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("git repos initialization failed: %w", err)
	}
	gitReposSvc = svc

	server := mcputil.CreateServer(mcputil.ServerConfig{
		Name:        "relic-mcp",
		Version:     "1.0.0",
		GitReposSvc: gitReposSvc,
	})

	return server, cleanup, nil
}
