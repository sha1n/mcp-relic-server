package gitrepos

import (
	"path/filepath"
	"strings"
)

// DefaultExcludePatterns contains file patterns to exclude from indexing.
// These patterns match common dependency directories, build outputs,
// generated files, and binary/media files that should not be searched.
var DefaultExcludePatterns = []string{
	// Dependencies
	"node_modules/**", "vendor/**", "venv/**", ".venv/**",
	"target/**", "build/**", "dist/**", "out/**",
	".git/**", "__pycache__/**", ".pytest_cache/**",
	".gradle/**", ".m2/**", ".npm/**", ".yarn/**",

	// Generated files
	"*.min.js", "*.min.css", "*.map", "*.pb.go",
	"package-lock.json", "yarn.lock", "pnpm-lock.yaml",
	"go.sum", "poetry.lock", "Cargo.lock",

	// Binary/Media - images
	"*.png", "*.jpg", "*.jpeg", "*.gif", "*.ico", "*.svg",
	"*.bmp", "*.tiff", "*.webp", "*.psd",

	// Binary/Media - fonts
	"*.woff", "*.woff2", "*.ttf", "*.eot", "*.otf",

	// Binary/Media - archives
	"*.zip", "*.tar", "*.gz", "*.rar", "*.7z", "*.bz2", "*.xz",
	"*.jar", "*.war", "*.ear",

	// Binary/Media - executables and libraries
	"*.exe", "*.dll", "*.so", "*.dylib", "*.a", "*.lib",
	"*.class", "*.pyc", "*.pyo", "*.o", "*.obj",

	// Binary/Media - documents
	"*.pdf", "*.doc", "*.docx", "*.xls", "*.xlsx", "*.ppt", "*.pptx",

	// Binary/Media - other
	"*.db", "*.sqlite", "*.sqlite3",
	"*.mp3", "*.mp4", "*.wav", "*.avi", "*.mov", "*.mkv",
}

// FileFilter determines which files should be included in indexing.
type FileFilter struct {
	patterns    []string
	maxFileSize int64
}

// NewFileFilter creates a new FileFilter with default exclusion patterns.
func NewFileFilter(maxFileSize int64) *FileFilter {
	return &FileFilter{
		patterns:    DefaultExcludePatterns,
		maxFileSize: maxFileSize,
	}
}

// NewFileFilterWithPatterns creates a FileFilter with custom patterns.
func NewFileFilterWithPatterns(patterns []string, maxFileSize int64) *FileFilter {
	return &FileFilter{
		patterns:    patterns,
		maxFileSize: maxFileSize,
	}
}

// ShouldExclude returns true if the given path matches any exclusion pattern.
// The path should be relative to the repository root.
func (f *FileFilter) ShouldExclude(relPath string) bool {
	// Normalize path separators
	relPath = filepath.ToSlash(relPath)

	for _, pattern := range f.patterns {
		if matchPattern(pattern, relPath) {
			return true
		}
	}
	return false
}

// MaxFileSize returns the maximum file size for indexing.
func (f *FileFilter) MaxFileSize() int64 {
	return f.maxFileSize
}

// matchPattern matches a file path against a glob pattern.
// Supports ** for directory matching and * for filename matching.
func matchPattern(pattern, path string) bool {
	// Handle **/ prefix (match any directory depth)
	if strings.HasPrefix(pattern, "**/") {
		// Match at root or any subdirectory
		rest := pattern[3:]
		if matchSimplePattern(rest, path) {
			return true
		}
		// Try matching at any subdirectory level
		parts := strings.Split(path, "/")
		for i := range parts {
			subPath := strings.Join(parts[i:], "/")
			if matchSimplePattern(rest, subPath) {
				return true
			}
		}
		return false
	}

	// Handle /** suffix (match directory and all contents)
	if strings.HasSuffix(pattern, "/**") {
		dir := pattern[:len(pattern)-3]
		// Match if path starts with dir/ or equals dir
		// Also check if dir appears anywhere in the path
		if path == dir || strings.HasPrefix(path, dir+"/") {
			return true
		}
		// Check if the directory appears as a component anywhere in the path
		if strings.Contains(path, "/"+dir+"/") || strings.HasPrefix(path, dir+"/") {
			return true
		}
		// Check each path component
		parts := strings.Split(path, "/")
		for i, part := range parts {
			if part == dir && i < len(parts)-1 {
				// This part matches the directory name, and there are more parts after
				return true
			}
		}
		return false
	}

	// Simple pattern matching
	return matchSimplePattern(pattern, path)
}

// matchSimplePattern matches a simple glob pattern (with * but not **).
func matchSimplePattern(pattern, name string) bool {
	// Handle patterns that start with *.
	if strings.HasPrefix(pattern, "*.") {
		ext := pattern[1:] // ".ext"
		return strings.HasSuffix(strings.ToLower(name), strings.ToLower(ext))
	}

	// Exact match
	if pattern == name {
		return true
	}

	// Match pattern against the filename (not full path) for extension patterns
	if strings.HasPrefix(pattern, "*") {
		baseName := filepath.Base(name)
		suffix := pattern[1:]
		return strings.HasSuffix(strings.ToLower(baseName), strings.ToLower(suffix))
	}

	// Use filepath.Match for other patterns
	matched, _ := filepath.Match(pattern, name)
	if matched {
		return true
	}

	// Also try matching against just the filename
	baseName := filepath.Base(name)
	matched, _ = filepath.Match(pattern, baseName)
	return matched
}

// IsBinary checks if the content appears to be binary by looking for null bytes
// in the first 512 bytes. This is a heuristic used by git and other tools.
func IsBinary(content []byte) bool {
	checkLen := min(len(content), 512)

	for i := range checkLen {
		if content[i] == 0 {
			return true
		}
	}
	return false
}

// IsTextFile checks if the content appears to be a text file.
// This is the inverse of IsBinary.
func IsTextFile(content []byte) bool {
	return !IsBinary(content)
}

// GetFileExtension returns the file extension without the leading dot.
// Returns empty string if no extension.
func GetFileExtension(path string) string {
	ext := filepath.Ext(path)
	return strings.TrimPrefix(ext, ".")
}
