package gitrepos

import (
	"slices"
	"testing"
)

func TestNewFileFilter(t *testing.T) {
	maxSize := int64(256 * 1024)
	filter := NewFileFilter(maxSize)

	if filter.MaxFileSize() != maxSize {
		t.Errorf("MaxFileSize() = %d, want %d", filter.MaxFileSize(), maxSize)
	}

	if len(filter.patterns) == 0 {
		t.Error("Expected default patterns to be set")
	}
}

func TestNewFileFilterWithPatterns(t *testing.T) {
	patterns := []string{"*.txt", "temp/**"}
	maxSize := int64(1024)

	filter := NewFileFilterWithPatterns(patterns, maxSize)

	if filter.MaxFileSize() != maxSize {
		t.Errorf("MaxFileSize() = %d, want %d", filter.MaxFileSize(), maxSize)
	}

	if len(filter.patterns) != 2 {
		t.Errorf("Expected 2 patterns, got %d", len(filter.patterns))
	}
}

func TestFileFilter_ShouldExclude_NodeModules(t *testing.T) {
	filter := NewFileFilter(256 * 1024)

	tests := []struct {
		path    string
		exclude bool
	}{
		{"node_modules/package/index.js", true},
		{"node_modules/deep/nested/file.js", true},
		{"src/node_modules/fake.js", true}, // nested node_modules
		{"src/index.js", false},
		{"nodemodules/file.js", false}, // different name, no underscore
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := filter.ShouldExclude(tt.path)
			if result != tt.exclude {
				t.Errorf("ShouldExclude(%q) = %v, want %v", tt.path, result, tt.exclude)
			}
		})
	}
}

func TestFileFilter_ShouldExclude_Vendor(t *testing.T) {
	filter := NewFileFilter(256 * 1024)

	tests := []struct {
		path    string
		exclude bool
	}{
		{"vendor/github.com/pkg/file.go", true},
		{"vendor/deep/nested/module/file.go", true},
		{"src/vendor/fake.go", true}, // nested vendor
		{"vendoring/file.go", false}, // different name
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := filter.ShouldExclude(tt.path)
			if result != tt.exclude {
				t.Errorf("ShouldExclude(%q) = %v, want %v", tt.path, result, tt.exclude)
			}
		})
	}
}

func TestFileFilter_ShouldExclude_Venv(t *testing.T) {
	filter := NewFileFilter(256 * 1024)

	tests := []struct {
		path    string
		exclude bool
	}{
		{"venv/lib/python3.9/site-packages/pkg.py", true},
		{".venv/bin/python", true},
		{"src/venv/file.py", true},
		{"myvenv/file.py", false}, // different name
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := filter.ShouldExclude(tt.path)
			if result != tt.exclude {
				t.Errorf("ShouldExclude(%q) = %v, want %v", tt.path, result, tt.exclude)
			}
		})
	}
}

func TestFileFilter_ShouldExclude_BuildDirs(t *testing.T) {
	filter := NewFileFilter(256 * 1024)

	tests := []struct {
		path    string
		exclude bool
	}{
		{"target/classes/Main.class", true},
		{"build/libs/app.jar", true},
		{"dist/bundle.js", true},
		{"out/production/Main.class", true},
		{"output/file.txt", false}, // different name
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := filter.ShouldExclude(tt.path)
			if result != tt.exclude {
				t.Errorf("ShouldExclude(%q) = %v, want %v", tt.path, result, tt.exclude)
			}
		})
	}
}

func TestFileFilter_ShouldExclude_GitDir(t *testing.T) {
	filter := NewFileFilter(256 * 1024)

	tests := []struct {
		path    string
		exclude bool
	}{
		{".git/config", true},
		{".git/objects/pack/file", true},
		{".git/HEAD", true},
		{".github/workflows/ci.yml", false}, // not .git
		{".gitignore", false},               // not .git directory
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := filter.ShouldExclude(tt.path)
			if result != tt.exclude {
				t.Errorf("ShouldExclude(%q) = %v, want %v", tt.path, result, tt.exclude)
			}
		})
	}
}

func TestFileFilter_ShouldExclude_BinaryExtensions(t *testing.T) {
	filter := NewFileFilter(256 * 1024)

	tests := []struct {
		path    string
		exclude bool
	}{
		// Images
		{"images/logo.png", true},
		{"assets/photo.jpg", true},
		{"assets/photo.JPEG", true}, // case insensitive
		{"icons/icon.gif", true},
		{"favicon.ico", true},
		{"diagram.svg", true},

		// Fonts
		{"fonts/roboto.woff", true},
		{"fonts/roboto.woff2", true},
		{"fonts/arial.ttf", true},

		// Archives
		{"release.zip", true},
		{"backup.tar", true},
		{"data.gz", true},

		// Executables
		{"app.exe", true},
		{"lib.dll", true},
		{"lib.so", true},
		{"lib.dylib", true},

		// Documents
		{"doc.pdf", true},
		{"report.docx", true},

		// Source files - should NOT be excluded
		{"main.go", false},
		{"app.py", false},
		{"index.js", false},
		{"style.css", false},
		{"README.md", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := filter.ShouldExclude(tt.path)
			if result != tt.exclude {
				t.Errorf("ShouldExclude(%q) = %v, want %v", tt.path, result, tt.exclude)
			}
		})
	}
}

func TestFileFilter_ShouldExclude_GeneratedFiles(t *testing.T) {
	filter := NewFileFilter(256 * 1024)

	tests := []struct {
		path    string
		exclude bool
	}{
		{"bundle.min.js", true},
		{"style.min.css", true},
		{"bundle.js.map", true},
		{"api.pb.go", true},
		{"package-lock.json", true},
		{"yarn.lock", true},
		{"pnpm-lock.yaml", true},
		{"go.sum", true},
		{"poetry.lock", true},
		{"Cargo.lock", true},

		// Regular files - not excluded
		{"bundle.js", false},
		{"style.css", false},
		{"package.json", false},
		{"go.mod", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := filter.ShouldExclude(tt.path)
			if result != tt.exclude {
				t.Errorf("ShouldExclude(%q) = %v, want %v", tt.path, result, tt.exclude)
			}
		})
	}
}

func TestFileFilter_ShouldExclude_NormalSourceFiles(t *testing.T) {
	filter := NewFileFilter(256 * 1024)

	// These should all be included (not excluded)
	paths := []string{
		"main.go",
		"src/main/java/App.java",
		"lib/utils.py",
		"components/Button.tsx",
		"styles/main.scss",
		"config/settings.yaml",
		"tests/test_utils.py",
		"README.md",
		"Makefile",
		"Dockerfile",
		".gitignore",
		".env.example",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			if filter.ShouldExclude(path) {
				t.Errorf("ShouldExclude(%q) = true, want false (source file should be included)", path)
			}
		})
	}
}

func TestFileFilter_ShouldExclude_NestedPaths(t *testing.T) {
	filter := NewFileFilter(256 * 1024)

	tests := []struct {
		path    string
		exclude bool
	}{
		// Nested node_modules should match
		{"frontend/node_modules/react/index.js", true},
		{"packages/web/node_modules/lodash/lodash.js", true},

		// Nested vendor should match
		{"services/api/vendor/lib/file.go", true},

		// Files with similar names but not matching patterns
		{"src/components/vendor_select.js", false},
		{"src/utils/node_modules_helper.js", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := filter.ShouldExclude(tt.path)
			if result != tt.exclude {
				t.Errorf("ShouldExclude(%q) = %v, want %v", tt.path, result, tt.exclude)
			}
		})
	}
}

func TestIsBinary_NullBytes(t *testing.T) {
	tests := []struct {
		name     string
		content  []byte
		isBinary bool
	}{
		{
			name:     "text content",
			content:  []byte("Hello, World!\n"),
			isBinary: false,
		},
		{
			name:     "text with unicode",
			content:  []byte("Hello, 世界! 🌍"),
			isBinary: false,
		},
		{
			name:     "null byte at start",
			content:  []byte{0x00, 'H', 'e', 'l', 'l', 'o'},
			isBinary: true,
		},
		{
			name:     "null byte in middle",
			content:  []byte{'H', 'e', 'l', 0x00, 'l', 'o'},
			isBinary: true,
		},
		{
			name:     "null byte at end (within 512)",
			content:  []byte{'H', 'e', 'l', 'l', 'o', 0x00},
			isBinary: true,
		},
		{
			name:     "empty content",
			content:  []byte{},
			isBinary: false,
		},
		{
			name:     "all null bytes",
			content:  []byte{0x00, 0x00, 0x00, 0x00},
			isBinary: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsBinary(tt.content)
			if result != tt.isBinary {
				t.Errorf("IsBinary() = %v, want %v", result, tt.isBinary)
			}
		})
	}
}

func TestIsBinary_LargeContent(t *testing.T) {
	// Content with null byte at position 600 (beyond 512 byte check)
	content := make([]byte, 1000)
	for i := range content {
		content[i] = 'a'
	}
	content[600] = 0x00

	if IsBinary(content) {
		t.Error("IsBinary() = true, want false (null byte beyond 512 check limit)")
	}

	// Content with null byte at position 100 (within 512 byte check)
	content[100] = 0x00
	if !IsBinary(content) {
		t.Error("IsBinary() = false, want true (null byte within 512 check limit)")
	}
}

func TestIsBinary_ShortContent(t *testing.T) {
	// Content shorter than 512 bytes
	content := []byte("short text")
	if IsBinary(content) {
		t.Error("IsBinary() = true, want false for short text content")
	}

	// Short content with null byte
	shortBinary := []byte{'a', 0x00, 'b'}
	if !IsBinary(shortBinary) {
		t.Error("IsBinary() = false, want true for short content with null byte")
	}
}

func TestIsTextFile(t *testing.T) {
	tests := []struct {
		name    string
		content []byte
		isText  bool
	}{
		{"regular text", []byte("Hello, World!"), true},
		{"empty", []byte{}, true},
		{"binary", []byte{0x00, 0x01, 0x02}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsTextFile(tt.content)
			if result != tt.isText {
				t.Errorf("IsTextFile() = %v, want %v", result, tt.isText)
			}
		})
	}
}

func TestGetFileExtension(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"main.go", "go"},
		{"app.py", "py"},
		{"styles.min.css", "css"},
		{"bundle.js.map", "map"},
		{"Makefile", ""},
		{"Dockerfile", ""},
		{".gitignore", "gitignore"},
		{".env.example", "example"},
		{"path/to/file.java", "java"},
		{"file", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := GetFileExtension(tt.path)
			if result != tt.expected {
				t.Errorf("GetFileExtension(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		path    string
		matches bool
	}{
		// Directory wildcards
		{"node_modules at root", "node_modules/**", "node_modules/file.js", true},
		{"node_modules nested", "node_modules/**", "node_modules/pkg/file.js", true},
		{"vendor at root", "vendor/**", "vendor/lib/file.go", true},

		// Extension patterns
		{"png extension", "*.png", "image.png", true},
		{"png in path", "*.png", "assets/image.png", true},
		{"png case insensitive", "*.png", "IMAGE.PNG", true},
		{"not png", "*.png", "image.jpg", false},

		// Exact match
		{"exact match", "package-lock.json", "package-lock.json", true},
		{"exact match in path", "package-lock.json", "pkg/package-lock.json", true},
		{"no match", "package-lock.json", "package.json", false},

		// Edge cases
		{"empty_pattern", "", "file.txt", false},
		{"pattern_no_match", "*.txt", "file.go", false},
		// New cases for coverage
		{"complex_glob", "test_?.go", "test_1.go", true},
		{"complex_glob_fail", "test_?.go", "test_10.go", false},
		{"bad_pattern", "[", "file.go", false},
		{"bad_pattern_simple", "a[", "file.go", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchPattern(tt.pattern, tt.path)
			if result != tt.matches {
				t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.pattern, tt.path, result, tt.matches)
			}
		})
	}
}

func TestDefaultExcludePatterns(t *testing.T) {
	// Verify default patterns are non-empty
	if len(DefaultExcludePatterns) == 0 {
		t.Fatal("DefaultExcludePatterns should not be empty")
	}

	// Verify key patterns exist
	expectedPatterns := []string{
		"node_modules/**",
		"vendor/**",
		".git/**",
		"*.png",
		"*.exe",
		"go.sum",
	}

	for _, expected := range expectedPatterns {
		if !slices.Contains(DefaultExcludePatterns, expected) {
			t.Errorf("Expected pattern %q not found in DefaultExcludePatterns", expected)
		}
	}
}
