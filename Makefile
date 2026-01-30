ifndef VERSION
VERSION := $(shell git describe --always --abbrev=0 --tags --match "v*" 2>/dev/null || echo "v0.0.0")
BUILD := $(shell git rev-parse --short HEAD 2>/dev/null || echo "HEAD")
endif
PROJECTNAME := "relic-mcp"
PROGRAMNAME := $(PROJECTNAME)

# Go related variables
GOHOSTOS := $(shell go env GOHOSTOS)
GOHOSTARCH := $(shell go env GOHOSTARCH)
GOBASE := $(shell pwd)
GOBIN := $(GOBASE)/bin
GOBUILD := $(GOBASE)/build
GOFILES := $(shell find . -type f -name '*.go' -not -path './vendor/*' -not -path './build/*')
GOOS_DARWIN := "darwin"
GOOS_LINUX := "linux"
GOOS_WINDOWS := "windows"
GOARCH_AMD64 := "amd64"
GOARCH_ARM64 := "arm64"
GOARCH_ARM := "arm"

MODFLAGS=-mod=readonly

# Linker flags for version injection
LDFLAGS=-ldflags "-w -s -X=main.Version=$(VERSION) -X=main.Build=$(BUILD) -X=main.ProgramName=$(PROGRAMNAME)"

.PHONY: default
default: install lint format test build

## install: Checks for missing dependencies and installs them
.PHONY: install
install: go-get

## format: Formats Go source files
.PHONY: format
format: go-format

## lint: Runs all linters including go vet, golangci-lint and format check
.PHONY: lint
lint: go-lint golangci-lint go-format-check

## build: Builds binaries for all supported platforms
.PHONY: build
build:
	@[ -d $(GOBUILD) ] || mkdir -p $(GOBUILD)
	@-mkdir -p $(GOBUILD)/completions
	@-$(MAKE) go-build
	$(GOBIN)/$(PROGRAMNAME)-$(GOHOSTOS)-$(GOHOSTARCH) completion zsh > $(GOBUILD)/completions/_$(PROGRAMNAME) || true
	$(GOBIN)/$(PROGRAMNAME)-$(GOHOSTOS)-$(GOHOSTARCH) completion bash > $(GOBUILD)/completions/$(PROGRAMNAME).bash || true
	$(GOBIN)/$(PROGRAMNAME)-$(GOHOSTOS)-$(GOHOSTARCH) completion fish > $(GOBUILD)/completions/$(PROGRAMNAME).fish || true

## go-build-current: Builds binary for current platform only
.PHONY: go-build-current
go-build-current:
	@echo "  >  Building $(GOHOSTOS)/$(GOHOSTARCH) binaries..."
	@GOOS=$(GOHOSTOS) GOARCH=$(GOHOSTARCH) GOBIN=$(GOBIN) go build $(MODFLAGS) $(LDFLAGS) -o $(GOBIN)/$(PROGRAMNAME) $(GOBASE)/cmd/relic-mcp

## test: Runs all Go tests
.PHONY: test
test: install go-test

## coverage: Runs all Go tests and generates a coverage report
.PHONY: coverage
coverage: install
	@echo "  >  Running tests with coverage (bypassing cache)..."
	go test $(MODFLAGS) -count=1 -coverpkg=./... -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

## coverage-html: Runs tests and opens the coverage report in a browser
.PHONY: coverage-html
coverage-html: coverage
	go tool cover -html=coverage.out

## clean: Removes build artifacts
.PHONY: clean
clean:
	@-rm $(GOBIN)/$(PROGRAMNAME)* 2> /dev/null
	@-$(MAKE) go-clean

.PHONY: go-lint
go-lint:
	@echo "  >  Linting source files..."
	go vet $(MODFLAGS) -c=10 `go list $(MODFLAGS) ./...`

## golangci-lint: Runs golangci-lint
.PHONY: golangci-lint
golangci-lint:
	@echo "  >  Running golangci-lint..."
	golangci-lint run

.PHONY: go-format
go-format:
	@echo "  >  Formatting source files..."
	gofmt -s -w $(GOFILES)

.PHONY: go-format-check
go-format-check:
	@echo "  >  Checking formatting of source files..."
	@if [ -n "$$(gofmt -l $(GOFILES))" ]; then \
		echo "  >  Format check failed for the following files:"; \
		gofmt -l $(GOFILES); \
		exit 1; \
	fi

.PHONY: go-build
go-build: go-get go-build-linux-amd64 go-build-linux-arm64 go-build-linux-arm go-build-darwin-amd64 go-build-darwin-arm64 go-build-windows-amd64 go-build-windows-arm

.PHONY: go-build-linux-amd64
go-build-linux-amd64:
	@echo "  >  Building linux amd64 binaries..."
	@GOOS=$(GOOS_LINUX) GOARCH=$(GOARCH_AMD64) GOBIN=$(GOBIN) go build $(MODFLAGS) $(LDFLAGS) -o $(GOBIN)/$(PROGRAMNAME)-$(GOOS_LINUX)-$(GOARCH_AMD64) $(GOBASE)/cmd/relic-mcp

.PHONY: go-build-linux-arm64
go-build-linux-arm64:
	@echo "  >  Building linux arm64 binaries..."
	@GOOS=$(GOOS_LINUX) GOARCH=$(GOARCH_ARM64) GOBIN=$(GOBIN) go build $(MODFLAGS) $(LDFLAGS) -o $(GOBIN)/$(PROGRAMNAME)-$(GOOS_LINUX)-$(GOARCH_ARM64) $(GOBASE)/cmd/relic-mcp

.PHONY: go-build-linux-arm
go-build-linux-arm:
	@echo "  >  Building linux arm binaries..."
	@GOOS=$(GOOS_LINUX) GOARCH=$(GOARCH_ARM) GOBIN=$(GOBIN) go build $(MODFLAGS) $(LDFLAGS) -o $(GOBIN)/$(PROGRAMNAME)-$(GOOS_LINUX)-$(GOARCH_ARM) $(GOBASE)/cmd/relic-mcp

.PHONY: go-build-darwin-amd64
go-build-darwin-amd64:
	@echo "  >  Building darwin amd64 binaries..."
	@GOOS=$(GOOS_DARWIN) GOARCH=$(GOARCH_AMD64) GOBIN=$(GOBIN) go build $(MODFLAGS) $(LDFLAGS) -o $(GOBIN)/$(PROGRAMNAME)-$(GOOS_DARWIN)-$(GOARCH_AMD64) $(GOBASE)/cmd/relic-mcp

.PHONY: go-build-darwin-arm64
go-build-darwin-arm64:
	@echo "  >  Building darwin arm64 binaries..."
	@GOOS=$(GOOS_DARWIN) GOARCH=$(GOARCH_ARM64) GOBIN=$(GOBIN) go build $(MODFLAGS) $(LDFLAGS) -o $(GOBIN)/$(PROGRAMNAME)-$(GOOS_DARWIN)-$(GOARCH_ARM64) $(GOBASE)/cmd/relic-mcp

.PHONY: go-build-windows-amd64
go-build-windows-amd64:
	@echo "  >  Building windows amd64 binaries..."
	@GOOS=$(GOOS_WINDOWS) GOARCH=$(GOARCH_AMD64) GOBIN=$(GOBIN) go build $(MODFLAGS) $(LDFLAGS) -o $(GOBIN)/$(PROGRAMNAME)-$(GOOS_WINDOWS)-$(GOARCH_AMD64).exe $(GOBASE)/cmd/relic-mcp

.PHONY: go-build-windows-arm
go-build-windows-arm:
	@echo "  >  Building windows arm binaries..."
	@GOOS=$(GOOS_WINDOWS) GOARCH=$(GOARCH_ARM) GOBIN=$(GOBIN) go build $(MODFLAGS) $(LDFLAGS) -o $(GOBIN)/$(PROGRAMNAME)-$(GOOS_WINDOWS)-$(GOARCH_ARM).exe $(GOBASE)/cmd/relic-mcp

.PHONY: go-get
go-get:
	@echo "  >  Checking if there is any missing dependencies..."
	@GOBIN=$(GOBIN) go mod tidy

.PHONY: go-test
go-test:
	@echo "  >  Running tests..."
	go test $(MODFLAGS) ./...

.PHONY: go-clean
go-clean:
	@echo "  >  Cleaning build cache"
	@GOBIN=$(GOBIN) go clean $(MODFLAGS) $(GOBASE)/cmd/relic-mcp
	@GOBIN=$(GOBIN) go clean -modcache

.PHONY: build-docker
build-docker:
	@echo "  >  Building docker image..."
	docker build -t sha1n/mcp-relic-server:latest .
	docker tag sha1n/mcp-relic-server:latest sha1n/mcp-relic-server:$(VERSION:v%=%)

## mcp-add-claude-dev: Adds the latest binaries from ./bin as relic-dev to Claude Code
.PHONY: mcp-add-claude-dev
mcp-add-claude-dev: go-build-current
	@echo "  >  Adding relic-dev to Claude Code..."
	claude mcp add --transport stdio relic-dev $(GOBIN)/$(PROGRAMNAME)

## mcp-remove-claude-dev: Removes relic-dev from Claude Code
.PHONY: mcp-remove-claude-dev
mcp-remove-claude-dev:
	@echo "  >  Removing relic-dev from Claude Code..."
	claude mcp remove relic-dev

.PHONY: all
all: help

.PHONY: help
help: Makefile
	@echo
	@echo " Choose a command run in "$(PROJECTNAME)":"
	@echo
	@sed -n 's/^##//p' $< | column -t -s ':' |  sed -e 's/^/ /'
	@echo
