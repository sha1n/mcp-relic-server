# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies
# git: needed for git describe (versioning) and go mod download
# make: needed if we were to use the Makefile, but we'll run go build directly for better control
RUN apk add --no-cache git make

WORKDIR /app

# Copy go module files first to cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
# We use CGO_ENABLED=0 for a static binary
# We try to extract version info similar to the Makefile
RUN VERSION=$(git describe --always --abbrev=0 --tags --match "v*" 2>/dev/null || echo "v0.0.0") && \
    BUILD=$(git rev-parse --short HEAD 2>/dev/null || echo "HEAD") && \
    CGO_ENABLED=0 go build \
    -ldflags="-w -s -X=main.Version=${VERSION} -X=main.Build=${BUILD} -X=main.ProgramName=relic-mcp" \
    -o /bin/relic-mcp ./cmd/relic-mcp

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
# git: required for git operations
# ca-certificates: required for HTTPS git clones
RUN apk add --no-cache git ca-certificates

# Create a non-root user
RUN addgroup -S relic && adduser -S relic -G relic

# Set working directory
WORKDIR /home/relic

# Create directory for git repos
RUN mkdir -p .relic-mcp && chown -R relic:relic .relic-mcp

# Copy binary from builder
COPY --from=builder /bin/relic-mcp /usr/local/bin/relic-mcp

# Switch to non-root user
USER relic

# Set environment variables
ENV RELIC_MCP_HOST=0.0.0.0 \
    RELIC_MCP_PORT=8080

# Expose the port
EXPOSE 8080

# Entrypoint
ENTRYPOINT ["relic-mcp"]
