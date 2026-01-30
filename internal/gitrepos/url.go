package gitrepos

import (
	"errors"
	"regexp"
	"strings"
)

var (
	// ErrInvalidSSHURL indicates the URL is not a valid SSH URL
	ErrInvalidSSHURL = errors.New("invalid SSH URL format")

	// Regex patterns for SSH URL parsing
	// Matches: git@github.com:org/repo.git or git@github.com:org/subgroup/repo.git
	sshScpPattern = regexp.MustCompile(`^git@([^:]+):(.+?)(?:\.git)?$`)

	// Matches: ssh://git@github.com/org/repo.git
	sshURLPattern = regexp.MustCompile(`^ssh://git@([^/]+)/(.+?)(?:\.git)?$`)
)

// ParseSSHURL parses an SSH git URL and returns the host, path, and repository name.
// Supports both SCP-style (git@host:path) and SSH URL style (ssh://git@host/path).
//
// Examples:
//   - git@github.com:org/repo.git -> host: github.com, path: org/repo, repo: repo
//   - git@gitlab.com:group/sub/repo.git -> host: gitlab.com, path: group/sub/repo, repo: repo
//   - ssh://git@github.com/org/repo.git -> host: github.com, path: org/repo, repo: repo
func ParseSSHURL(url string) (host, path, repo string, err error) {
	url = strings.TrimSpace(url)

	// Try SCP-style pattern first (more common)
	if matches := sshScpPattern.FindStringSubmatch(url); matches != nil {
		host = matches[1]
		path = matches[2]
		repo = extractRepoName(path)
		return host, path, repo, nil
	}

	// Try SSH URL pattern
	if matches := sshURLPattern.FindStringSubmatch(url); matches != nil {
		host = matches[1]
		path = matches[2]
		repo = extractRepoName(path)
		return host, path, repo, nil
	}

	return "", "", "", ErrInvalidSSHURL
}

// extractRepoName extracts the repository name from a path.
// For "org/repo" returns "repo", for "group/sub/repo" returns "repo".
func extractRepoName(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return path
}

// URLToRepoID converts an SSH URL to a filesystem-safe repository ID.
// The ID is used for directory names and index references.
//
// Examples:
//   - git@github.com:org/repo.git -> github.com_org_repo
//   - git@gitlab.com:group/sub/repo.git -> gitlab.com_group_sub_repo
//   - ssh://git@github.com/org/repo.git -> github.com_org_repo
func URLToRepoID(url string) string {
	host, path, _, err := ParseSSHURL(url)
	if err != nil {
		// Fallback: sanitize the URL directly
		return sanitizeForFilesystem(url)
	}

	// Combine host and path, replacing slashes with underscores
	combined := host + "/" + path
	return sanitizeForFilesystem(combined)
}

// RepoIDToDisplay converts a repository ID back to a display format.
// This is the inverse of URLToRepoID (approximately).
//
// Examples:
//   - github.com_org_repo -> github.com/org/repo
//   - gitlab.com_group_sub_repo -> gitlab.com/group/sub/repo
func RepoIDToDisplay(repoID string) string {
	// Find the first underscore (separates host from path)
	host, rest, found := strings.Cut(repoID, "_")
	if !found {
		return repoID
	}

	// Replace remaining underscores with slashes
	path := strings.ReplaceAll(rest, "_", "/")

	return host + "/" + path
}

// DisplayToRepoID converts a display format to a repository ID.
//
// Examples:
//   - github.com/org/repo -> github.com_org_repo
//   - gitlab.com/group/sub/repo -> gitlab.com_group_sub_repo
func DisplayToRepoID(display string) string {
	return sanitizeForFilesystem(display)
}

// sanitizeForFilesystem converts a string to a filesystem-safe format.
// Replaces slashes, colons, and @ symbols with underscores.
func sanitizeForFilesystem(s string) string {
	s = strings.TrimPrefix(s, "git@")
	s = strings.TrimPrefix(s, "ssh://git@")
	s = strings.TrimSuffix(s, ".git")
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, ":", "_")
	s = strings.ReplaceAll(s, "@", "_")
	return s
}

// IsValidSSHURL checks if the given URL is a valid SSH git URL.
func IsValidSSHURL(url string) bool {
	_, _, _, err := ParseSSHURL(url)
	return err == nil
}
