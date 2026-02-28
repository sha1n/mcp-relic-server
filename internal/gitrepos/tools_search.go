package gitrepos

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/blevesearch/bleve/v2"
	_ "github.com/blevesearch/bleve/v2/search/highlight/highlighter/ansi"
	"github.com/blevesearch/bleve/v2/search/query"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sha1n/mcp-relic-server/internal/domain"
)

// SearchArgument defines search parameters.
type SearchArgument struct {
	Query      string `json:"query" jsonschema_description:"Search query. Use natural language or keywords."`
	Repository string `json:"repository,omitempty" jsonschema_description:"Filter by repository name (substring match)"`
	Extension  string `json:"extension,omitempty" jsonschema_description:"Filter by file extension (e.g., 'go', 'py', 'java')"`
}

// SearchHandler handles the search MCP tool.
type SearchHandler struct {
	service SearchService
}

// NewSearchHandler creates a new search handler.
func NewSearchHandler(service SearchService) *SearchHandler {
	return &SearchHandler{
		service: service,
	}
}

// Handle executes the search and returns formatted results.
func (h *SearchHandler) Handle(ctx context.Context, req *mcp.CallToolRequest, args SearchArgument) (*mcp.CallToolResult, any, error) {
	// Check if service is ready
	if !h.service.IsReady() {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Search is not available. The git repositories are still being indexed. Please try again later."},
			},
			IsError: true,
		}, nil, nil
	}

	// Validate query
	if strings.TrimSpace(args.Query) == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Query cannot be empty"},
			},
			IsError: true,
		}, nil, nil
	}

	// Get index alias
	alias, err := h.service.GetIndexAlias()
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Failed to access indexes: %s", err)},
			},
			IsError: true,
		}, nil, nil
	}

	// Build query
	searchQuery := h.buildQuery(args)

	// Create search request
	searchReq := bleve.NewSearchRequest(searchQuery)
	searchReq.Size = h.service.MaxResults()
	searchReq.Fields = []string{domain.CodeFieldRepository, domain.CodeFieldFilePath, domain.CodeFieldExtension, domain.CodeFieldContent}
	searchReq.Highlight = bleve.NewHighlightWithStyle("ansi")
	searchReq.Highlight.AddField(domain.CodeFieldContent)

	// Execute search
	results, err := alias.Search(searchReq)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Search failed: %s", err)},
			},
			IsError: true,
		}, nil, nil
	}

	// Format results
	return h.formatResults(results, args.Query), nil, nil
}

// buildQuery constructs a Bleve query from search arguments.
func (h *SearchHandler) buildQuery(args SearchArgument) query.Query {
	// Content query
	contentQuery := bleve.NewMatchQuery(args.Query)
	contentQuery.SetField(domain.CodeFieldContent)
	contentQuery.SetFuzziness(1)

	// Symbols query with boost
	symbolsQuery := bleve.NewMatchQuery(args.Query)
	symbolsQuery.SetField(domain.CodeFieldSymbols)
	symbolsQuery.SetBoost(5.0)

	// Combined search query (Disjunction - OR)
	searchQuery := bleve.NewDisjunctionQuery(contentQuery, symbolsQuery)

	// If no filters, return search query directly
	if args.Repository == "" && args.Extension == "" {
		return searchQuery
	}

	// Build conjunction query with filters
	must := []query.Query{searchQuery}

	if args.Repository != "" {
		// Substring match on repository name using regexp with escaped input
		repoQuery := bleve.NewRegexpQuery(".*" + regexp.QuoteMeta(strings.ToLower(args.Repository)) + ".*")
		repoQuery.SetField(domain.CodeFieldRepository)
		must = append(must, repoQuery)
	}

	if args.Extension != "" {
		// Normalize extension (remove leading dot if present)
		ext := strings.TrimPrefix(args.Extension, ".")
		extQuery := bleve.NewTermQuery(ext)
		extQuery.SetField(domain.CodeFieldExtension)
		must = append(must, extQuery)
	}

	return bleve.NewConjunctionQuery(must...)
}

// formatResults formats Bleve search results for MCP response.
func (h *SearchHandler) formatResults(results *bleve.SearchResult, queryStr string) *mcp.CallToolResult {
	if results.Total == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("No results found for query: %s", queryStr)},
			},
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d results for '%s':\n\n", results.Total, queryStr))

	for i, hit := range results.Hits {
		// Extract fields
		repo := ""
		filePath := ""
		ext := ""
		if val, ok := hit.Fields[domain.CodeFieldRepository].(string); ok {
			repo = val
		}
		if val, ok := hit.Fields[domain.CodeFieldFilePath].(string); ok {
			filePath = val
		}
		if val, ok := hit.Fields[domain.CodeFieldExtension].(string); ok {
			ext = val
		}

		// Write result header
		sb.WriteString(fmt.Sprintf("**%d. %s** `%s`\n", i+1, repo, filePath))

		// Add highlighted fragments with language-specific code fencing
		if len(hit.Fragments) > 0 {
			if fragments, ok := hit.Fragments[domain.CodeFieldContent]; ok {
				lang := extensionToLanguage(ext)
				sb.WriteString(fmt.Sprintf("```%s\n", lang))
				for _, fragment := range fragments {
					sb.WriteString(fragment)
					sb.WriteString("\n")
				}
				sb.WriteString("```\n")
			}
		}

		sb.WriteString("\n")
	}

	if results.Total > uint64(len(results.Hits)) {
		sb.WriteString(fmt.Sprintf("... and %d more results\n", results.Total-uint64(len(results.Hits))))
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: sb.String()},
		},
	}
}

// GetToolDefinition returns the MCP tool definition.
func (h *SearchHandler) GetToolDefinition() *mcp.Tool {
	return &mcp.Tool{
		Name: "search",
		Description: `Search across indexed git repositories for code, documentation, and configuration.

WHEN TO USE: Use this to find implementation patterns, understand how features work
across the codebase, locate configuration files, or find usage examples.

HOW IT WORKS: Searches file content with optional filtering by repository or
file extension. Returns matching files with relevant code snippets.`,
	}
}

// RegisterSearchTool registers the search tool with an MCP server.
func RegisterSearchTool(server *mcp.Server, service SearchService) {
	handler := NewSearchHandler(service)
	mcp.AddTool(server, handler.GetToolDefinition(), handler.Handle)
}
