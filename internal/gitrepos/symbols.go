package gitrepos

import (
	"regexp"
	"strings"
)

// LanguageRegex defines patterns for a language
type LanguageRegex struct {
	Patterns []*regexp.Regexp
}

var languagePatterns = map[string]LanguageRegex{
	"go": {
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`func\s+(\w+)`),
			regexp.MustCompile(`type\s+(\w+)\s+(struct|interface)`),
			regexp.MustCompile(`const\s+(\w+)`),
			regexp.MustCompile(`var\s+(\w+)`),
		},
	},
	"py": {
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?m)^\s*def\s+(\w+)`),
			regexp.MustCompile(`(?m)^\s*class\s+(\w+)`),
		},
	},
	"python": {
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?m)^\s*def\s+(\w+)`),
			regexp.MustCompile(`(?m)^\s*class\s+(\w+)`),
		},
	},
	"java": {
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`class\s+(\w+)`),
			regexp.MustCompile(`interface\s+(\w+)`),
			regexp.MustCompile(`enum\s+(\w+)`),
			regexp.MustCompile(`(?:public|protected|private|static|\s) +[\w\<\>\[\]]+\s+(\w+) *\(`), // Method
		},
	},
	"js": {
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`function\s+(\w+)`),
			regexp.MustCompile(`class\s+(\w+)`),
			regexp.MustCompile(`const\s+(\w+)\s*=`),
			regexp.MustCompile(`let\s+(\w+)\s*=`),
			regexp.MustCompile(`var\s+(\w+)\s*=`),
		},
	},
	"ts": {
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`function\s+(\w+)`),
			regexp.MustCompile(`class\s+(\w+)`),
			regexp.MustCompile(`interface\s+(\w+)`),
			regexp.MustCompile(`type\s+(\w+)\s*=`),
			regexp.MustCompile(`const\s+(\w+)\s*=`),
			regexp.MustCompile(`let\s+(\w+)\s*=`),
		},
	},
	"rs": {
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`fn\s+(\w+)`),
			regexp.MustCompile(`struct\s+(\w+)`),
			regexp.MustCompile(`enum\s+(\w+)`),
			regexp.MustCompile(`trait\s+(\w+)`),
			regexp.MustCompile(`mod\s+(\w+)`),
			regexp.MustCompile(`type\s+(\w+)`),
		},
	},
	"c": {
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?m)^\s*\w+\s+(\w+)\s*\(.*\)\s*\{`), // Function definition
			regexp.MustCompile(`struct\s+(\w+)`),
			regexp.MustCompile(`enum\s+(\w+)`),
			regexp.MustCompile(`#define\s+(\w+)`),
		},
	},
	"cpp": {
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`class\s+(\w+)`),
			regexp.MustCompile(`struct\s+(\w+)`),
			regexp.MustCompile(`enum\s+(\w+)`),
			regexp.MustCompile(`(?m)^\s*\w+\s+(\w+)\s*\(.*\)\s*\{`), // Function definition (simplified)
		},
	},
}

// ExtractSymbols extracts symbols from content based on file extension.
func ExtractSymbols(ext, content string) []string {
	normalizedExt := strings.ToLower(strings.TrimPrefix(ext, "."))
	patterns, ok := languagePatterns[normalizedExt]
	if !ok {
		// Try mapping commonly used extensions
		switch normalizedExt {
		case "javascript", "jsx":
			patterns = languagePatterns["js"]
		case "typescript", "tsx":
			patterns = languagePatterns["ts"]
		case "golang":
			patterns = languagePatterns["go"]
		case "rust":
			patterns = languagePatterns["rs"]
		case "h":
			patterns = languagePatterns["c"]
		case "hpp", "cc", "cxx":
			patterns = languagePatterns["cpp"]
		default:
			return nil
		}
	}

	if len(patterns.Patterns) == 0 {
		return nil
	}

	uniqueSymbols := make(map[string]struct{})
	for _, regex := range patterns.Patterns {
		matches := regex.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if len(match) > 1 {
				// match[1] should be the identifier
				symbol := strings.TrimSpace(match[1])
				// Basic validation to ensure it looks like an identifier
				if symbol != "" && len(symbol) < 100 {
					uniqueSymbols[symbol] = struct{}{}
				}
			}
		}
	}

	if len(uniqueSymbols) == 0 {
		return nil
	}

	symbols := make([]string, 0, len(uniqueSymbols))
	for s := range uniqueSymbols {
		symbols = append(symbols, s)
	}
	return symbols
}
