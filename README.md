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

## Running

### Stdio Transport (default)

```bash
relic-mcp
```

### SSE Transport

```bash
relic-mcp --transport sse --port 8080
```

### Health Check (SSE Only)

The SSE server exposes an unauthenticated `/health` endpoint that returns `200 OK`.

## Configuration

| Flag          | Short | Environment Variable  | Default   |
| ------------- | ----- | --------------------- | --------- |
| `--transport` | `-t`  | `RELIC_MCP_TRANSPORT` | `stdio`   |
| `--host`      | `-H`  | `RELIC_MCP_HOST`      | `0.0.0.0` |
| `--port`      | `-p`  | `RELIC_MCP_PORT`      | `8080`    |
| `--auth-type` | `-a`  | `RELIC_MCP_AUTH_TYPE` | `none`    |

### Authentication

**Basic Auth:**
```bash
relic-mcp --transport sse --auth-type basic \
  --auth-basic-username admin \
  --auth-basic-password secret
```

**API Key:**
```bash
relic-mcp --transport sse --auth-type apikey \
  --auth-api-keys "key1,key2"
```

## Agent Configuration

### Claude Code

**Stdio:**
```bash
claude mcp add --scope user --transport stdio relic -- relic-mcp
```

**SSE:**
```bash
claude mcp add --scope user --transport sse relic http://<host>:<port>/sse
```

## Development

```bash
# Run tests
make test

# Run tests with coverage
make coverage

# Run linters
make lint

# Format code
make format
```

## License

MIT License
