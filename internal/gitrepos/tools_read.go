package gitrepos

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ReadArgument defines read parameters.
type ReadArgument struct {
	Repository string `json:"repository" jsonschema_description:"Repository name (e.g., github.com/org/repo)"`
	Path       string `json:"path" jsonschema_description:"File path relative to repository root"`
}

// ReadHandler handles the read MCP tool.
type ReadHandler struct {
	service *Service
}

// NewReadHandler creates a new read handler.
func NewReadHandler(service *Service) *ReadHandler {
	return &ReadHandler{
		service: service,
	}
}

// Handle reads a file and returns formatted content.
func (h *ReadHandler) Handle(ctx context.Context, req *mcp.CallToolRequest, args ReadArgument) (*mcp.CallToolResult, any, error) {
	// Check if service is ready
	if !h.service.IsReady() {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Read is not available. The git repositories are still being indexed. Please try again later."},
			},
			IsError: true,
		}, nil, nil
	}

	// Validate repository
	if strings.TrimSpace(args.Repository) == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Repository cannot be empty"},
			},
			IsError: true,
		}, nil, nil
	}

	// Validate path
	if strings.TrimSpace(args.Path) == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Path cannot be empty"},
			},
			IsError: true,
		}, nil, nil
	}

	// Validate path security
	if err := validatePath(args.Path); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Invalid path: %s", err)},
			},
			IsError: true,
		}, nil, nil
	}

	// Convert repository to repo ID
	repoID := DisplayToRepoID(args.Repository)
	repoDir := h.service.GetRepoDir(repoID)

	// Check if repo directory exists
	if _, err := os.Stat(repoDir); os.IsNotExist(err) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Repository not found: %s", args.Repository)},
			},
			IsError: true,
		}, nil, nil
	}

	// Build full path
	fullPath := filepath.Join(repoDir, filepath.Clean(args.Path))

	// Security check: ensure the path is within repo directory
	if !strings.HasPrefix(fullPath, repoDir) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Path traversal detected"},
			},
			IsError: true,
		}, nil, nil
	}

	// Check if file exists
	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: fmt.Sprintf("File not found: %s", args.Path)},
				},
				IsError: true,
			}, nil, nil
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error accessing file: %s", err)},
			},
			IsError: true,
		}, nil, nil
	}

	// Check if it's a directory
	if info.IsDir() {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Cannot read directory, please specify a file path"},
			},
			IsError: true,
		}, nil, nil
	}

	// Check file size
	maxFileSize := h.service.GetSettings().MaxFileSize
	if info.Size() > maxFileSize {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("File too large (%.2f KB). Maximum allowed size is %.2f KB", float64(info.Size())/1024, float64(maxFileSize)/1024)},
			},
			IsError: true,
		}, nil, nil
	}

	// Read file content
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error reading file: %s", err)},
			},
			IsError: true,
		}, nil, nil
	}

	// Check for binary content
	if IsBinary(content) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Cannot display binary file content"},
			},
			IsError: true,
		}, nil, nil
	}

	// Format result with language hint
	lang := extensionToLanguage(GetFileExtension(args.Path))
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("**File**: `%s`\n", args.Path))
	sb.WriteString(fmt.Sprintf("**Repository**: %s\n", args.Repository))
	sb.WriteString(fmt.Sprintf("**Size**: %d bytes\n\n", len(content)))
	sb.WriteString(fmt.Sprintf("```%s\n%s\n```", lang, string(content)))

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: sb.String()},
		},
	}, nil, nil
}

// validatePath performs security validation on the path.
func validatePath(path string) error {
	// Clean the path
	cleaned := filepath.Clean(path)

	// Reject absolute paths
	if filepath.IsAbs(cleaned) {
		return fmt.Errorf("absolute paths are not allowed")
	}

	// Reject paths that try to traverse up
	if strings.HasPrefix(cleaned, "..") || strings.Contains(cleaned, "/..") || strings.Contains(cleaned, "\\..") {
		return fmt.Errorf("path traversal is not allowed")
	}

	return nil
}

// extensionToLanguage maps file extension to language hint for code blocks.
func extensionToLanguage(ext string) string {
	langMap := map[string]string{
		"go":         "go",
		"py":         "python",
		"js":         "javascript",
		"ts":         "typescript",
		"jsx":        "jsx",
		"tsx":        "tsx",
		"java":       "java",
		"kt":         "kotlin",
		"rs":         "rust",
		"c":          "c",
		"cpp":        "cpp",
		"cc":         "cpp",
		"h":          "c",
		"hpp":        "cpp",
		"cs":         "csharp",
		"rb":         "ruby",
		"php":        "php",
		"swift":      "swift",
		"scala":      "scala",
		"sh":         "bash",
		"bash":       "bash",
		"zsh":        "zsh",
		"fish":       "fish",
		"ps1":        "powershell",
		"sql":        "sql",
		"html":       "html",
		"htm":        "html",
		"css":        "css",
		"scss":       "scss",
		"sass":       "sass",
		"less":       "less",
		"json":       "json",
		"yaml":       "yaml",
		"yml":        "yaml",
		"toml":       "toml",
		"xml":        "xml",
		"md":         "markdown",
		"txt":        "text",
		"proto":      "protobuf",
		"graphql":    "graphql",
		"gql":        "graphql",
		"tf":         "terraform",
		"dockerfile": "dockerfile",
	}

	if lang, ok := langMap[strings.ToLower(ext)]; ok {
		return lang
	}
	return ext
}

// GetToolDefinition returns the MCP tool definition.
func (h *ReadHandler) GetToolDefinition() *mcp.Tool {
	return &mcp.Tool{
		Name:        "read_code",
		Description: "Read a file from an indexed git repository",
	}
}

// RegisterReadTool registers the read tool with an MCP server.
func RegisterReadTool(server *mcp.Server, service *Service) {
	handler := NewReadHandler(service)
	mcp.AddTool(server, handler.GetToolDefinition(), handler.Handle)
}
