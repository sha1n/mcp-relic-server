# mcp-relic-server

**Repository Exploration and Lookup for Indexed Code (RELIC) MCP Server**

A Model Context Protocol (MCP) server for AI agents to search and read code across multiple Git repositories. Features full-text search powered by Bleve, dual transport support (stdio/SSE), and flexible authentication.

## Features

- **Full-Text Search** — Fast indexing with fuzzy matching across repositories
- **File Reading** — Direct file access with path traversal protection
- **MCP Compliant** — Seamless integration with AI agents
- **Dual Transport** — `stdio` for local agents, `sse` for remote/Docker
- **Authentication** — Optional basic auth or API key protection
- **Cross-Platform** — Linux, macOS, and Windows

## Installation

### Build from Source

```bash
git clone https://github.com/sha1n/mcp-relic-server.git
cd mcp-relic-server
make install
make go-build-current
```

The compiled binary is located at `bin/relic-mcp`.

### Docker

```bash
docker build -t relic-mcp .
docker run -p 8080:8080 relic-mcp --transport sse
```

### Docker Compose

A `docker-compose.yml` file is included for easy deployment.

1. Create a `data` directory for persistent storage:
   ```bash
   mkdir data
   ```

2. Edit `docker-compose.yml` to configure your repositories (or set via environment variables).

3. Run the server:
   ```bash
   docker-compose up -d
   ```

## Quick Start

### 1. Configure Git Repositories

Create a `.env` file or set environment variables:

```bash
# Enable git repository indexing
RELIC_MCP_GIT_REPOS_ENABLED=true

# Comma-separated list of SSH URLs to index
RELIC_MCP_GIT_REPOS_URLS=git@github.com:org/repo1.git,git@github.com:org/repo2.git
```

### 2. Run the Server

**Stdio (for local agents like Claude Code):**
```bash
relic-mcp
```

**SSE (for remote access or Docker):**
```bash
relic-mcp --transport sse --port 8080
```

### 3. Connect Your Agent

See [Agent Configuration](#agent-configuration) below.

---

## Configuration Reference

All settings can be configured via environment variables, CLI flags, or `.env` file.

### Transport Settings

| Flag | Env Variable | Default | Description |
|------|--------------|---------|-------------|
| `--transport`, `-t` | `RELIC_MCP_TRANSPORT` | `stdio` | Transport mode: `stdio` or `sse` |
| `--host`, `-H` | `RELIC_MCP_HOST` | `0.0.0.0` | Host to bind (SSE only) |
| `--port`, `-p` | `RELIC_MCP_PORT` | `8080` | Port to bind (SSE only) |

### Authentication Settings (SSE only)

| Flag | Env Variable | Default | Description |
|------|--------------|---------|-------------|
| `--auth-type`, `-a` | `RELIC_MCP_AUTH_TYPE` | `none` | Auth type: `none`, `basic`, or `apikey` |
| `--auth-basic-username` | `RELIC_MCP_AUTH_BASIC_USERNAME` | | Username for basic auth |
| `--auth-basic-password` | `RELIC_MCP_AUTH_BASIC_PASSWORD` | | Password for basic auth |
| `--auth-api-keys` | `RELIC_MCP_AUTH_API_KEYS` | | Comma-separated API keys |

### Git Repository Settings

| Flag | Env Variable | Default | Description |
|------|--------------|---------|-------------|
| `--git-repos-enabled` | `RELIC_MCP_GIT_REPOS_ENABLED` | `false` | Enable git repository indexing |
| `--git-repos-urls` | `RELIC_MCP_GIT_REPOS_URLS` | | Comma-separated SSH URLs |
| `--git-repos-base-dir` | `RELIC_MCP_GIT_REPOS_BASE_DIR` | `~/.relic-mcp` | Base directory for clones and indexes |
| `--git-repos-sync-interval` | `RELIC_MCP_GIT_REPOS_SYNC_INTERVAL` | `15m` | Minimum interval between syncs |
| `--git-repos-sync-timeout` | `RELIC_MCP_GIT_REPOS_SYNC_TIMEOUT` | `60s` | Max time to wait for sync lock |
| `--git-repos-max-file-size` | `RELIC_MCP_GIT_REPOS_MAX_FILE_SIZE` | `262144` | Max file size to index (bytes, default 256KB) |
| `--git-repos-max-results` | `RELIC_MCP_GIT_REPOS_MAX_RESULTS` | `20` | Max search results to return |

---

## Transport Modes

### Stdio Transport (Default)

Best for local AI agents that spawn the server as a subprocess.

```bash
# Basic usage
relic-mcp

# With git repos enabled
relic-mcp --git-repos-enabled --git-repos-urls "git@github.com:org/repo.git"
```

**Characteristics:**
- Server communicates via stdin/stdout
- One server instance per agent session
- Multiple instances coordinate via file locking
- Indexes are shared via mmap (memory efficient)

### SSE Transport

Best for remote access, Docker deployments, or shared server setups.

```bash
# Basic SSE server
relic-mcp --transport sse --port 8080

# With authentication
relic-mcp --transport sse --port 8080 --auth-type basic \
  --auth-basic-username admin --auth-basic-password secret

# With API key auth
relic-mcp --transport sse --port 8080 --auth-type apikey \
  --auth-api-keys "key1,key2,key3"

# Full configuration
relic-mcp --transport sse \
  --host 0.0.0.0 \
  --port 8080 \
  --auth-type apikey \
  --auth-api-keys "your-secret-key" \
  --git-repos-enabled \
  --git-repos-urls "git@github.com:org/repo1.git,git@github.com:org/repo2.git"
```

**Endpoints:**
- `/sse` — MCP SSE endpoint
- `/health` — Health check (unauthenticated, returns `200 OK`)

**Characteristics:**
- HTTP-based Server-Sent Events
- Single server instance serves multiple clients
- Optional authentication (basic or API key)
- Suitable for Docker and Kubernetes deployments

---

## Agent Configuration

### Claude Code (Stdio)

Add the MCP server to Claude Code:

```bash
# Using the CLI
claude mcp add --scope user --transport stdio relic -- relic-mcp \
  --git-repos-enabled \
  --git-repos-urls "git@github.com:your-org/your-repo.git"
```

Or add manually to `~/.claude/settings.json`:

```json
{
  "mcpServers": {
    "relic": {
      "command": "relic-mcp",
      "args": [
        "--git-repos-enabled",
        "--git-repos-urls", "git@github.com:org/repo1.git,git@github.com:org/repo2.git"
      ]
    }
  }
}
```

### Claude Code (SSE)

For remote SSE servers:

```bash
claude mcp add --scope user --transport sse relic http://localhost:8080/sse
```

Or with authentication header:

```bash
claude mcp add --scope user --transport sse \
  --header "Authorization: Bearer your-api-key" \
  relic http://localhost:8080/sse
```

### Other MCP Clients

For any MCP-compatible client, use:

**Stdio:**
- Command: `relic-mcp`
- Args: `["--git-repos-enabled", "--git-repos-urls", "git@github.com:org/repo.git"]`

**SSE:**
- URL: `http://<host>:<port>/sse`
- Headers: `{"Authorization": "Bearer <api-key>"}` (if auth enabled)

---

## MCP Tools

When git repository indexing is enabled, the following tools are available:

### `search_code`

Search for code across indexed repositories.

**Arguments:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `query` | string | Yes | Search query (keywords or natural language) |
| `repository` | string | No | Filter by repository name (e.g., `github.com/org/repo`) |
| `extension` | string | No | Filter by file extension (e.g., `go`, `py`, `js`) |

**Example:**
```json
{
  "query": "authentication middleware",
  "repository": "github.com/org/api-server",
  "extension": "go"
}
```

### `read_code`

Read the full content of a file from an indexed repository.

**Arguments:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `repository` | string | Yes | Repository name (e.g., `github.com/org/repo`) |
| `path` | string | Yes | File path relative to repository root |

**Example:**
```json
{
  "repository": "github.com/org/api-server",
  "path": "src/middleware/auth.go"
}
```

---

## Example Configurations

### Local Development (Stdio)

`.env` file:
```bash
RELIC_MCP_GIT_REPOS_ENABLED=true
RELIC_MCP_GIT_REPOS_URLS=git@github.com:myorg/frontend.git,git@github.com:myorg/backend.git
RELIC_MCP_GIT_REPOS_BASE_DIR=~/.relic-mcp
RELIC_MCP_GIT_REPOS_MAX_RESULTS=30
```

Run:
```bash
relic-mcp
```

### Docker Deployment (SSE with Auth)

`docker-compose.yml`:
```yaml
version: '3.8'
services:
  relic-mcp:
    build: .
    ports:
      - "8080:8080"
    environment:
      - RELIC_MCP_TRANSPORT=sse
      - RELIC_MCP_PORT=8080
      - RELIC_MCP_AUTH_TYPE=apikey
      - RELIC_MCP_AUTH_API_KEYS=your-secret-api-key
      - RELIC_MCP_GIT_REPOS_ENABLED=true
      - RELIC_MCP_GIT_REPOS_URLS=git@github.com:org/repo1.git,git@github.com:org/repo2.git
    volumes:
      - relic-data:/root/.relic-mcp
      - ~/.ssh:/root/.ssh:ro  # Mount SSH keys for git access
volumes:
  relic-data:
```

### Team Server (SSE with Basic Auth)

```bash
relic-mcp \
  --transport sse \
  --host 0.0.0.0 \
  --port 443 \
  --auth-type basic \
  --auth-basic-username team \
  --auth-basic-password "secure-password" \
  --git-repos-enabled \
  --git-repos-urls "git@github.com:company/monorepo.git" \
  --git-repos-sync-interval 5m \
  --git-repos-max-results 50
```

---

## How It Works

### Repository Synchronization

1. On startup, RELIC clones configured repositories (shallow clone, single branch)
2. Subsequent starts fetch and reset to latest HEAD
3. Multiple instances coordinate via file locking (leader/follower model)
4. Indexes are stored on disk and shared via mmap across processes

### File Filtering

The following are automatically excluded from indexing:

- **Dependencies**: `node_modules/`, `vendor/`, `venv/`, `target/`, `build/`, `dist/`
- **Generated files**: `*.min.js`, `*.min.css`, `*.pb.go`, lock files
- **Binary/Media**: Images, fonts, archives, executables, PDFs
- **Git internals**: `.git/` directory

Binary files are also detected by content (null bytes in first 512 bytes).

### Security

- **Path traversal prevention**: All file paths are validated and sanitized
- **Repository isolation**: Only configured repositories are accessible
- **File size limits**: Large files are rejected to prevent memory exhaustion
- **Binary detection**: Binary files cannot be read via the `read_code` tool

---

## Development

```bash
# Install dependencies
make install

# Run tests
make test

# Run tests with coverage
make coverage

# Run linters
make lint

# Format code
make format

# Build for current platform
make go-build-current

# Build for all platforms
make build

# Build Docker image
make build-docker
```

---

## Troubleshooting

### "Code search is not available"

The git repos service is still initializing. Wait a few seconds and try again.

### "Repository not found"

Check that:
1. The repository URL is correctly configured
2. SSH keys are set up for the repository
3. The repository name matches exactly (e.g., `github.com/org/repo`)

### "Sync timeout"

Another instance is currently syncing. This is normal for the first startup after adding new repositories. Subsequent queries will use the existing index.

### SSH Authentication Issues

Ensure your SSH configuration is correct:
```bash
# Test SSH access
ssh -T git@github.com

# Check SSH agent
ssh-add -l
```

---

## License

MIT License
