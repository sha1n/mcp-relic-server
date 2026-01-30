package domain

// CodeDocument represents an indexed source file in a git repository.
// It is the primary data structure stored in the Bleve search index.
type CodeDocument struct {
	// ID is a unique identifier combining repo ID and file path.
	// Format: "github.com_org_repo/path/to/file.go"
	ID string `json:"id"`

	// Repository is the human-readable repository identifier.
	// Format: "github.com/org/repo"
	Repository string `json:"repository"`

	// FilePath is the file path relative to the repository root.
	// Example: "src/main/java/App.java"
	FilePath string `json:"file_path"`

	// Extension is the file extension without the leading dot.
	// Example: "java", "go", "py"
	Extension string `json:"extension"`

	// Content is the full file content used for indexing and search snippets.
	Content string `json:"content"`
}

// Bleve field name constants for consistent field references in queries and mappings.
const (
	CodeFieldID         = "id"
	CodeFieldRepository = "repository"
	CodeFieldFilePath   = "file_path"
	CodeFieldExtension  = "extension"
	CodeFieldContent    = "content"
)
