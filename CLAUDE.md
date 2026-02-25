# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

RELIC (Repository Exploration and Lookup for Indexed Code) is an MCP (Model Context Protocol) server written in Go. It provides full-text search and file reading capabilities across multiple Git repositories for AI agents.

## Common Commands

```bash
# Install dependencies
make install

# Build for current platform
make go-build-current

# Build for all platforms
make build

# Run all tests
make test

# Run tests with coverage
make coverage

# Run linters (go vet, golangci-lint, format check)
make lint

# Format code
make format

# Build Docker image
make build-docker

# Run a single test
go test -v ./internal/config -run TestLoadSettings

# Add local dev build to Claude Code for testing
make mcp-add-claude-dev
```

## Architecture

### Package Structure

- `cmd/relic-mcp/` - CLI entry point using Cobra
- `internal/app/` - Application orchestration (runner, SSE server setup)
- `internal/mcp/` - MCP server implementation using the official `modelcontextprotocol/go-sdk`
- `internal/config/` - Settings and configuration (env vars, CLI flags, .env files)
- `internal/gitrepos/` - Git repository indexing, search, and file reading (Bleve-based)
- `internal/auth/` - Authentication middleware (basic auth, API key)
- `tests/integration/` - Integration tests with testkit utilities

### Key Flows

**Startup**: `main.go` → `app.RunWithDeps()` → initializes git repos service → creates MCP server → starts transport (stdio or SSE)

**Transports**: Supports `stdio` (default, local process) and `sse` (HTTP with optional auth). Mutually exclusive.

### Key Interfaces

- `mcp.Server` - The MCP server from the official SDK
- `config.Settings` - Application configuration loaded from env/flags/.env
- `auth.NewMiddleware()` - Creates HTTP middleware for authentication
- `gitrepos.SearchService` / `gitrepos.ReadService` - Narrow interfaces for MCP tool handlers
- `gitrepos.GitOperations`, `IndexOperations`, `ManifestOperations`, `SyncLock` - Component interfaces for dependency injection
- `mcp.GitReposToolService` - Combined interface used by the MCP server layer

### Gitrepos Architecture

The `internal/gitrepos/` package uses **consumer-defined interfaces** (Go convention). Interfaces are defined in `interfaces.go` at the consumer level, not alongside implementations.

- `Service` struct holds interface fields (`GitOperations`, `IndexOperations`, etc.) instead of concrete types
- `NewService()` creates production implementations; `NewServiceWithDeps()` accepts injected mocks for testing
- `symbols.go` extracts code symbols (functions, types, classes) via regex patterns per language, boosting search relevance
- Handler tests use mocks for validation logic; integration tests use real Bleve indexes for search behavior

### Testing

Tests use dependency injection patterns for easy mocking. The `RunParams` struct in `app/runner.go` allows injecting test dependencies.

Use table-driven tests for comprehensive coverage of edge cases.

```bash
# Run all tests
make test

# Run tests with coverage
make coverage

# Run a single test
go test -v ./internal/config -run TestLoadSettings

# Run integration tests only
go test -v ./tests/integration/...
```

#### Integration Test Kit

Integration tests use a testkit (`tests/integration/testkit/`) that provides:

- **TestEnv** - Interface for managing test service lifecycle (Start/Stop)
- **Service** - Interface for test services with Start/Stop/GetName methods
- **GetFreePort()** / **MustGetFreePort(t)** - Port allocation utilities
- **NewTestFlags(t, opts)** - Creates configured pflag.FlagSet for testing

Example usage:

```go
import "github.com/sha1n/mcp-relic-server/tests/integration/testkit"

func TestMyIntegration(t *testing.T) {
    // Get a free port for the test server
    port := testkit.MustGetFreePort(t)

    // Create test flags with custom options
    flags := testkit.NewTestFlags(t, &testkit.FlagOptions{
        Port:      port,
        Transport: "sse",
        AuthType:  "none",
    })

    // Use flags with config.LoadSettingsWithFlags()
    settings, err := config.LoadSettingsWithFlags(flags)
    // ...
}
```
