package domain

import (
	"encoding/json"
	"testing"
)

func TestCodeDocument_JSONMarshal(t *testing.T) {
	doc := CodeDocument{
		ID:         "github.com_org_repo/src/main.go",
		Repository: "github.com/org/repo",
		FilePath:   "src/main.go",
		Extension:  "go",
		Content:    "package main\n\nfunc main() {}\n",
	}

	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("Failed to marshal CodeDocument: %v", err)
	}

	var decoded CodeDocument
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal CodeDocument: %v", err)
	}

	if decoded.ID != doc.ID {
		t.Errorf("ID mismatch: got %q, want %q", decoded.ID, doc.ID)
	}
	if decoded.Repository != doc.Repository {
		t.Errorf("Repository mismatch: got %q, want %q", decoded.Repository, doc.Repository)
	}
	if decoded.FilePath != doc.FilePath {
		t.Errorf("FilePath mismatch: got %q, want %q", decoded.FilePath, doc.FilePath)
	}
	if decoded.Extension != doc.Extension {
		t.Errorf("Extension mismatch: got %q, want %q", decoded.Extension, doc.Extension)
	}
	if decoded.Content != doc.Content {
		t.Errorf("Content mismatch: got %q, want %q", decoded.Content, doc.Content)
	}
}

func TestCodeDocument_JSONUnmarshal(t *testing.T) {
	jsonData := `{
		"id": "github.com_acme_project/lib/utils.py",
		"repository": "github.com/acme/project",
		"file_path": "lib/utils.py",
		"extension": "py",
		"content": "def helper():\n    pass\n"
	}`

	var doc CodeDocument
	if err := json.Unmarshal([]byte(jsonData), &doc); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if doc.ID != "github.com_acme_project/lib/utils.py" {
		t.Errorf("Unexpected ID: %q", doc.ID)
	}
	if doc.Repository != "github.com/acme/project" {
		t.Errorf("Unexpected Repository: %q", doc.Repository)
	}
	if doc.FilePath != "lib/utils.py" {
		t.Errorf("Unexpected FilePath: %q", doc.FilePath)
	}
	if doc.Extension != "py" {
		t.Errorf("Unexpected Extension: %q", doc.Extension)
	}
	if doc.Content != "def helper():\n    pass\n" {
		t.Errorf("Unexpected Content: %q", doc.Content)
	}
}

func TestCodeDocument_EmptyFields(t *testing.T) {
	doc := CodeDocument{}

	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("Failed to marshal empty CodeDocument: %v", err)
	}

	var decoded CodeDocument
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal empty CodeDocument: %v", err)
	}

	if decoded.ID != "" || decoded.Repository != "" || decoded.FilePath != "" ||
		decoded.Extension != "" || decoded.Content != "" {
		t.Error("Expected all fields to be empty strings")
	}
}

func TestCodeFieldConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{"CodeFieldID", CodeFieldID, "id"},
		{"CodeFieldRepository", CodeFieldRepository, "repository"},
		{"CodeFieldFilePath", CodeFieldFilePath, "file_path"},
		{"CodeFieldExtension", CodeFieldExtension, "extension"},
		{"CodeFieldContent", CodeFieldContent, "content"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("%s = %q, want %q", tt.name, tt.constant, tt.expected)
			}
		})
	}
}

func TestCodeDocument_JSONFieldNames(t *testing.T) {
	doc := CodeDocument{
		ID:         "test-id",
		Repository: "test-repo",
		FilePath:   "test-path",
		Extension:  "txt",
		Content:    "test content",
	}

	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Verify JSON field names match the constants
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Failed to unmarshal to map: %v", err)
	}

	expectedFields := map[string]string{
		CodeFieldID:         "test-id",
		CodeFieldRepository: "test-repo",
		CodeFieldFilePath:   "test-path",
		CodeFieldExtension:  "txt",
		CodeFieldContent:    "test content",
	}

	for field, expected := range expectedFields {
		if val, ok := raw[field]; !ok {
			t.Errorf("Missing field %q in JSON output", field)
		} else if val != expected {
			t.Errorf("Field %q = %v, want %v", field, val, expected)
		}
	}
}
