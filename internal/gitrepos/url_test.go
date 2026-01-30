package gitrepos

import (
	"errors"
	"testing"
)

func TestParseSSHURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		wantHost string
		wantPath string
		wantRepo string
		wantErr  error
	}{
		{
			name:     "standard github scp style with .git",
			url:      "git@github.com:org/repo.git",
			wantHost: "github.com",
			wantPath: "org/repo",
			wantRepo: "repo",
			wantErr:  nil,
		},
		{
			name:     "standard github scp style without .git",
			url:      "git@github.com:org/repo",
			wantHost: "github.com",
			wantPath: "org/repo",
			wantRepo: "repo",
			wantErr:  nil,
		},
		{
			name:     "gitlab with subgroup",
			url:      "git@gitlab.com:group/subgroup/repo.git",
			wantHost: "gitlab.com",
			wantPath: "group/subgroup/repo",
			wantRepo: "repo",
			wantErr:  nil,
		},
		{
			name:     "gitlab with multiple subgroups",
			url:      "git@gitlab.com:group/sub1/sub2/repo.git",
			wantHost: "gitlab.com",
			wantPath: "group/sub1/sub2/repo",
			wantRepo: "repo",
			wantErr:  nil,
		},
		{
			name:     "bitbucket",
			url:      "git@bitbucket.org:team/project.git",
			wantHost: "bitbucket.org",
			wantPath: "team/project",
			wantRepo: "project",
			wantErr:  nil,
		},
		{
			name:     "ssh url style with .git",
			url:      "ssh://git@github.com/org/repo.git",
			wantHost: "github.com",
			wantPath: "org/repo",
			wantRepo: "repo",
			wantErr:  nil,
		},
		{
			name:     "ssh url style without .git",
			url:      "ssh://git@github.com/org/repo",
			wantHost: "github.com",
			wantPath: "org/repo",
			wantRepo: "repo",
			wantErr:  nil,
		},
		{
			name:     "url with whitespace",
			url:      "  git@github.com:org/repo.git  ",
			wantHost: "github.com",
			wantPath: "org/repo",
			wantRepo: "repo",
			wantErr:  nil,
		},
		{
			name:     "custom git server",
			url:      "git@git.company.com:team/project.git",
			wantHost: "git.company.com",
			wantPath: "team/project",
			wantRepo: "project",
			wantErr:  nil,
		},
		{
			name:    "invalid https url",
			url:     "https://github.com/org/repo.git",
			wantErr: ErrInvalidSSHURL,
		},
		{
			name:    "invalid http url",
			url:     "http://github.com/org/repo.git",
			wantErr: ErrInvalidSSHURL,
		},
		{
			name:    "empty string",
			url:     "",
			wantErr: ErrInvalidSSHURL,
		},
		{
			name:    "random string",
			url:     "not a url at all",
			wantErr: ErrInvalidSSHURL,
		},
		{
			name:    "missing path",
			url:     "git@github.com:",
			wantErr: ErrInvalidSSHURL,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotHost, gotPath, gotRepo, err := ParseSSHURL(tt.url)

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("ParseSSHURL(%q) expected error %v, got nil", tt.url, tt.wantErr)
					return
				}
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("ParseSSHURL(%q) error = %v, want %v", tt.url, err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("ParseSSHURL(%q) unexpected error: %v", tt.url, err)
				return
			}

			if gotHost != tt.wantHost {
				t.Errorf("ParseSSHURL(%q) host = %q, want %q", tt.url, gotHost, tt.wantHost)
			}
			if gotPath != tt.wantPath {
				t.Errorf("ParseSSHURL(%q) path = %q, want %q", tt.url, gotPath, tt.wantPath)
			}
			if gotRepo != tt.wantRepo {
				t.Errorf("ParseSSHURL(%q) repo = %q, want %q", tt.url, gotRepo, tt.wantRepo)
			}
		})
	}
}

func TestURLToRepoID(t *testing.T) {
	tests := []struct {
		name   string
		url    string
		wantID string
	}{
		{
			name:   "github scp style",
			url:    "git@github.com:org/repo.git",
			wantID: "github.com_org_repo",
		},
		{
			name:   "github scp style without .git",
			url:    "git@github.com:org/repo",
			wantID: "github.com_org_repo",
		},
		{
			name:   "gitlab with subgroups",
			url:    "git@gitlab.com:group/subgroup/repo.git",
			wantID: "gitlab.com_group_subgroup_repo",
		},
		{
			name:   "ssh url style",
			url:    "ssh://git@github.com/org/repo.git",
			wantID: "github.com_org_repo",
		},
		{
			name:   "bitbucket",
			url:    "git@bitbucket.org:team/project.git",
			wantID: "bitbucket.org_team_project",
		},
		{
			name:   "invalid url fallback",
			url:    "https://github.com/org/repo",
			wantID: "https___github.com_org_repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotID := URLToRepoID(tt.url)
			if gotID != tt.wantID {
				t.Errorf("URLToRepoID(%q) = %q, want %q", tt.url, gotID, tt.wantID)
			}
		})
	}
}

func TestRepoIDToDisplay(t *testing.T) {
	tests := []struct {
		name        string
		repoID      string
		wantDisplay string
	}{
		{
			name:        "github repo",
			repoID:      "github.com_org_repo",
			wantDisplay: "github.com/org/repo",
		},
		{
			name:        "gitlab with subgroups",
			repoID:      "gitlab.com_group_subgroup_repo",
			wantDisplay: "gitlab.com/group/subgroup/repo",
		},
		{
			name:        "simple repo",
			repoID:      "host_path",
			wantDisplay: "host/path",
		},
		{
			name:        "no underscore",
			repoID:      "nounderscore",
			wantDisplay: "nounderscore",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotDisplay := RepoIDToDisplay(tt.repoID)
			if gotDisplay != tt.wantDisplay {
				t.Errorf("RepoIDToDisplay(%q) = %q, want %q", tt.repoID, gotDisplay, tt.wantDisplay)
			}
		})
	}
}

func TestDisplayToRepoID(t *testing.T) {
	tests := []struct {
		name    string
		display string
		wantID  string
	}{
		{
			name:    "github repo",
			display: "github.com/org/repo",
			wantID:  "github.com_org_repo",
		},
		{
			name:    "gitlab with subgroups",
			display: "gitlab.com/group/subgroup/repo",
			wantID:  "gitlab.com_group_subgroup_repo",
		},
		{
			name:    "no slashes",
			display: "simple",
			wantID:  "simple",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotID := DisplayToRepoID(tt.display)
			if gotID != tt.wantID {
				t.Errorf("DisplayToRepoID(%q) = %q, want %q", tt.display, gotID, tt.wantID)
			}
		})
	}
}

func TestRoundTrip_URLToRepoIDToDisplay(t *testing.T) {
	// Test that URLToRepoID -> RepoIDToDisplay produces a reasonable display name
	tests := []struct {
		name        string
		url         string
		wantDisplay string
	}{
		{
			name:        "github",
			url:         "git@github.com:org/repo.git",
			wantDisplay: "github.com/org/repo",
		},
		{
			name:        "gitlab with subgroups",
			url:         "git@gitlab.com:group/sub/repo.git",
			wantDisplay: "gitlab.com/group/sub/repo",
		},
		{
			name:        "ssh url style",
			url:         "ssh://git@github.com/org/repo.git",
			wantDisplay: "github.com/org/repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoID := URLToRepoID(tt.url)
			display := RepoIDToDisplay(repoID)
			if display != tt.wantDisplay {
				t.Errorf("URLToRepoID(%q) -> RepoIDToDisplay() = %q, want %q", tt.url, display, tt.wantDisplay)
			}
		})
	}
}

func TestIsValidSSHURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		isValid bool
	}{
		{"valid scp style", "git@github.com:org/repo.git", true},
		{"valid ssh url style", "ssh://git@github.com/org/repo.git", true},
		{"invalid https", "https://github.com/org/repo.git", false},
		{"invalid empty", "", false},
		{"invalid random", "not a url", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidSSHURL(tt.url)
			if got != tt.isValid {
				t.Errorf("IsValidSSHURL(%q) = %v, want %v", tt.url, got, tt.isValid)
			}
		})
	}
}

func TestSanitizeForFilesystem(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		output string
	}{
		{
			name:   "removes git@ prefix",
			input:  "git@github.com:org/repo.git",
			output: "github.com_org_repo",
		},
		{
			name:   "removes ssh://git@ prefix",
			input:  "ssh://git@github.com/org/repo.git",
			output: "github.com_org_repo",
		},
		{
			name:   "removes .git suffix",
			input:  "github.com/org/repo.git",
			output: "github.com_org_repo",
		},
		{
			name:   "replaces slashes",
			input:  "github.com/org/repo",
			output: "github.com_org_repo",
		},
		{
			name:   "replaces colons",
			input:  "github.com:org:repo",
			output: "github.com_org_repo",
		},
		{
			name:   "replaces at symbols",
			input:  "user@github.com/repo",
			output: "user_github.com_repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeForFilesystem(tt.input)
			if got != tt.output {
				t.Errorf("sanitizeForFilesystem(%q) = %q, want %q", tt.input, got, tt.output)
			}
		})
	}
}

func TestExtractRepoName(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		result string
	}{
		{"simple", "org/repo", "repo"},
		{"with subgroups", "group/sub/repo", "repo"},
		{"single element", "repo", "repo"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractRepoName(tt.path)
			if got != tt.result {
				t.Errorf("extractRepoName(%q) = %q, want %q", tt.path, got, tt.result)
			}
		})
	}
}
