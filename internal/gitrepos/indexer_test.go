package gitrepos

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/blevesearch/bleve/v2"
	"github.com/sha1n/mcp-relic-server/internal/domain"
)

// closeIndex is a helper to close an index in tests and fail on error
func closeIndex(t *testing.T, idx io.Closer) {
	t.Helper()
	if err := idx.Close(); err != nil {
		t.Errorf("Failed to close index: %v", err)
	}
}

func TestNewIndexer(t *testing.T) {
	filter := NewFileFilter(256 * 1024)
	indexer := NewIndexer("/tmp/test", filter, 256*1024)

	if indexer.baseDir != "/tmp/test" {
		t.Errorf("baseDir = %q, want '/tmp/test'", indexer.baseDir)
	}
	if indexer.maxFileSize != 256*1024 {
		t.Errorf("maxFileSize = %d", indexer.maxFileSize)
	}
}

func TestIndexer_OpenForWrite_New(t *testing.T) {
	dir := t.TempDir()
	filter := NewFileFilter(256 * 1024)
	indexer := NewIndexer(dir, filter, 256*1024)

	index, err := indexer.OpenForWrite("testrepo")
	if err != nil {
		t.Fatalf("OpenForWrite failed: %v", err)
	}
	defer closeIndex(t, index)

	// Verify index was created
	indexPath := filepath.Join(dir, "indexes", "testrepo.bleve")
	if _, err := os.Stat(indexPath); err != nil {
		t.Errorf("Index directory should exist: %v", err)
	}
}

func TestIndexer_OpenForWrite_Existing(t *testing.T) {
	dir := t.TempDir()
	filter := NewFileFilter(256 * 1024)
	indexer := NewIndexer(dir, filter, 256*1024)

	// Create index
	index1, err := indexer.OpenForWrite("testrepo")
	if err != nil {
		t.Fatalf("First OpenForWrite failed: %v", err)
	}

	// Index a document
	doc := domain.CodeDocument{
		ID:         "testrepo/test.go",
		Repository: "testrepo",
		FilePath:   "test.go",
		Extension:  "go",
		Content:    "package main",
	}
	if err := index1.Index(doc.ID, doc); err != nil {
		t.Fatalf("Index failed: %v", err)
	}
	closeIndex(t, index1)

	// Reopen index
	index2, err := indexer.OpenForWrite("testrepo")
	if err != nil {
		t.Fatalf("Second OpenForWrite failed: %v", err)
	}
	defer closeIndex(t, index2)

	// Verify document still exists
	count, err := index2.DocCount()
	if err != nil {
		t.Fatalf("DocCount failed: %v", err)
	}
	if count != 1 {
		t.Errorf("DocCount = %d, want 1", count)
	}
}

func TestIndexer_OpenForRead(t *testing.T) {
	dir := t.TempDir()
	filter := NewFileFilter(256 * 1024)
	indexer := NewIndexer(dir, filter, 256*1024)

	// Create index first
	index, err := indexer.OpenForWrite("testrepo")
	if err != nil {
		t.Fatalf("OpenForWrite failed: %v", err)
	}
	closeIndex(t, index)

	// Open for read
	readIndex, err := indexer.OpenForRead("testrepo")
	if err != nil {
		t.Fatalf("OpenForRead failed: %v", err)
	}
	defer closeIndex(t, readIndex)
}

func TestIndexer_OpenForRead_NonExistent(t *testing.T) {
	dir := t.TempDir()
	filter := NewFileFilter(256 * 1024)
	indexer := NewIndexer(dir, filter, 256*1024)

	_, err := indexer.OpenForRead("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent index")
	}
}

func TestIndexer_IndexExists(t *testing.T) {
	dir := t.TempDir()
	filter := NewFileFilter(256 * 1024)
	indexer := NewIndexer(dir, filter, 256*1024)

	// Should not exist initially
	if indexer.IndexExists("testrepo") {
		t.Error("Index should not exist initially")
	}

	// Create index
	index, err := indexer.OpenForWrite("testrepo")
	if err != nil {
		t.Fatalf("OpenForWrite failed: %v", err)
	}
	closeIndex(t, index)

	// Should exist now
	if !indexer.IndexExists("testrepo") {
		t.Error("Index should exist after creation")
	}
}

func TestIndexer_CreateAlias(t *testing.T) {
	dir := t.TempDir()
	filter := NewFileFilter(256 * 1024)
	indexer := NewIndexer(dir, filter, 256*1024)

	// Create two indexes
	for _, repoID := range []string{"repo1", "repo2"} {
		index, err := indexer.OpenForWrite(repoID)
		if err != nil {
			t.Fatalf("OpenForWrite failed: %v", err)
		}
		doc := domain.CodeDocument{
			ID:         repoID + "/file.go",
			Repository: repoID,
			FilePath:   "file.go",
			Extension:  "go",
			Content:    "package " + repoID,
		}
		if err := index.Index(doc.ID, doc); err != nil {
			t.Fatalf("Index failed: %v", err)
		}
		closeIndex(t, index)
	}

	// Create alias
	alias, err := indexer.CreateAlias([]string{"repo1", "repo2"})
	if err != nil {
		t.Fatalf("CreateAlias failed: %v", err)
	}
	defer closeIndex(t, alias)

	// Search across both indexes
	query := bleve.NewMatchQuery("package")
	searchReq := bleve.NewSearchRequest(query)
	searchReq.Size = 10

	results, err := alias.Search(searchReq)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if results.Total != 2 {
		t.Errorf("Expected 2 results from alias, got %d", results.Total)
	}
}

func TestIndexer_CreateAlias_Empty(t *testing.T) {
	dir := t.TempDir()
	filter := NewFileFilter(256 * 1024)
	indexer := NewIndexer(dir, filter, 256*1024)

	_, err := indexer.CreateAlias([]string{})
	if err == nil {
		t.Error("Expected error for empty alias")
	}
}

func TestIndexer_CreateAlias_NonExistent(t *testing.T) {
	dir := t.TempDir()
	filter := NewFileFilter(256 * 1024)
	indexer := NewIndexer(dir, filter, 256*1024)

	_, err := indexer.CreateAlias([]string{"nonexistent"})
	if err == nil {
		t.Error("Expected error for non-existent repo")
	}
}

func TestIndexer_FullIndex(t *testing.T) {
	dir := t.TempDir()
	repoDir := filepath.Join(dir, "repos", "testrepo")
	filter := NewFileFilter(256 * 1024)
	indexer := NewIndexer(dir, filter, 256*1024)

	// Create test files
	createTestFile(t, repoDir, "main.go", "package main\nfunc main() {}")
	createTestFile(t, repoDir, "lib/utils.go", "package lib\nfunc Helper() {}")
	createTestFile(t, repoDir, "README.md", "# Test Repository")

	// Run full index
	count, err := indexer.FullIndex("testrepo", repoDir)
	if err != nil {
		t.Fatalf("FullIndex failed: %v", err)
	}

	if count != 3 {
		t.Errorf("Expected 3 files indexed, got %d", count)
	}

	// Verify search works
	index, err := indexer.OpenForRead("testrepo")
	if err != nil {
		t.Fatalf("OpenForRead failed: %v", err)
	}
	defer closeIndex(t, index)

	query := bleve.NewMatchQuery("main")
	searchReq := bleve.NewSearchRequest(query)
	results, err := index.Search(searchReq)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if results.Total == 0 {
		t.Error("Expected search results for 'main'")
	}
}

func TestIndexer_FullIndex_IncludesSymbols(t *testing.T) {
	dir := t.TempDir()
	repoDir := filepath.Join(dir, "repos", "testrepo")
	filter := NewFileFilter(256 * 1024)
	indexer := NewIndexer(dir, filter, 256*1024)

	// Create test files with symbols
	createTestFile(t, repoDir, "main.go", "package main\nfunc MySpecialFunction() {}")

	// Run full index
	_, err := indexer.FullIndex("testrepo", repoDir)
	if err != nil {
		t.Fatalf("FullIndex failed: %v", err)
	}

	// Verify search works against symbols field specifically
	index, err := indexer.OpenForRead("testrepo")
	if err != nil {
		t.Fatalf("OpenForRead failed: %v", err)
	}
	defer closeIndex(t, index)

	// Create a query specifically for symbols field
	query := bleve.NewMatchQuery("MySpecialFunction")
	query.SetField(domain.CodeFieldSymbols)
	searchReq := bleve.NewSearchRequest(query)
	results, err := index.Search(searchReq)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if results.Total == 0 {
		t.Error("Expected search results for 'MySpecialFunction' in symbols field")
	}
}

func TestIndexer_FullIndex_SkipsExcluded(t *testing.T) {
	dir := t.TempDir()
	repoDir := filepath.Join(dir, "repos", "testrepo")
	filter := NewFileFilter(256 * 1024)
	indexer := NewIndexer(dir, filter, 256*1024)

	// Create test files including excluded ones
	createTestFile(t, repoDir, "main.go", "package main")
	createTestFile(t, repoDir, "node_modules/pkg/index.js", "module.exports = {}")
	createTestFile(t, repoDir, "vendor/lib/lib.go", "package lib")
	createTestFile(t, repoDir, "image.png", "fake binary content")

	count, err := indexer.FullIndex("testrepo", repoDir)
	if err != nil {
		t.Fatalf("FullIndex failed: %v", err)
	}

	// Should only index main.go (node_modules, vendor, and .png are excluded)
	if count != 1 {
		t.Errorf("Expected 1 file indexed (main.go), got %d", count)
	}
}

func TestIndexer_FullIndex_SkipsLargeFiles(t *testing.T) {
	dir := t.TempDir()
	repoDir := filepath.Join(dir, "repos", "testrepo")
	filter := NewFileFilter(100) // Very small max size
	indexer := NewIndexer(dir, filter, 100)

	// Create test files
	createTestFile(t, repoDir, "small.go", "package main") // ~12 bytes
	createTestFile(t, repoDir, "large.go", makeLargeContent(200))

	count, err := indexer.FullIndex("testrepo", repoDir)
	if err != nil {
		t.Fatalf("FullIndex failed: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 file indexed (small only), got %d", count)
	}
}

func TestIndexer_FullIndex_SkipsBinary(t *testing.T) {
	dir := t.TempDir()
	repoDir := filepath.Join(dir, "repos", "testrepo")
	filter := NewFileFilter(256 * 1024)
	indexer := NewIndexer(dir, filter, 256*1024)

	// Create test files
	createTestFile(t, repoDir, "text.go", "package main")
	createBinaryFile(t, repoDir, "binary.dat")

	count, err := indexer.FullIndex("testrepo", repoDir)
	if err != nil {
		t.Fatalf("FullIndex failed: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 file indexed (text only), got %d", count)
	}
}

func TestIndexer_FullIndex_SkipsGitDir(t *testing.T) {
	dir := t.TempDir()
	repoDir := filepath.Join(dir, "repos", "testrepo")
	filter := NewFileFilter(256 * 1024)
	indexer := NewIndexer(dir, filter, 256*1024)

	// Create test files
	createTestFile(t, repoDir, "main.go", "package main")
	createTestFile(t, repoDir, ".git/config", "[core]")
	createTestFile(t, repoDir, ".git/HEAD", "ref: refs/heads/main")

	count, err := indexer.FullIndex("testrepo", repoDir)
	if err != nil {
		t.Fatalf("FullIndex failed: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 file indexed (main.go only), got %d", count)
	}
}

func TestIndexer_IncrementalIndex_AddNew(t *testing.T) {
	dir := t.TempDir()
	repoDir := filepath.Join(dir, "repos", "testrepo")
	filter := NewFileFilter(256 * 1024)
	indexer := NewIndexer(dir, filter, 256*1024)

	// Create initial file and index
	createTestFile(t, repoDir, "main.go", "package main")
	_, err := indexer.FullIndex("testrepo", repoDir)
	if err != nil {
		t.Fatalf("FullIndex failed: %v", err)
	}

	// Add new file
	createTestFile(t, repoDir, "new.go", "package new")

	// Incremental index
	count, err := indexer.IncrementalIndex("testrepo", repoDir, []string{"new.go"})
	if err != nil {
		t.Fatalf("IncrementalIndex failed: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 file indexed, got %d", count)
	}

	// Verify both files are in index
	docCount, err := indexer.GetDocumentCount("testrepo")
	if err != nil {
		t.Fatalf("GetDocumentCount failed: %v", err)
	}
	if docCount != 2 {
		t.Errorf("Expected 2 documents total, got %d", docCount)
	}
}

func TestIndexer_IncrementalIndex_Update(t *testing.T) {
	dir := t.TempDir()
	repoDir := filepath.Join(dir, "repos", "testrepo")
	filter := NewFileFilter(256 * 1024)
	indexer := NewIndexer(dir, filter, 256*1024)

	// Create initial file and index
	createTestFile(t, repoDir, "main.go", "package main\n// version 1")
	_, err := indexer.FullIndex("testrepo", repoDir)
	if err != nil {
		t.Fatalf("FullIndex failed: %v", err)
	}

	// Update file
	createTestFile(t, repoDir, "main.go", "package main\n// version 2")

	// Incremental index
	count, err := indexer.IncrementalIndex("testrepo", repoDir, []string{"main.go"})
	if err != nil {
		t.Fatalf("IncrementalIndex failed: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 file indexed, got %d", count)
	}

	// Verify updated content is searchable
	index, err := indexer.OpenForRead("testrepo")
	if err != nil {
		t.Fatalf("OpenForRead failed: %v", err)
	}
	defer closeIndex(t, index)

	query := bleve.NewMatchQuery("version 2")
	searchReq := bleve.NewSearchRequest(query)
	results, err := index.Search(searchReq)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if results.Total == 0 {
		t.Error("Expected to find updated content")
	}
}

func TestIndexer_IncrementalIndex_Delete(t *testing.T) {
	dir := t.TempDir()
	repoDir := filepath.Join(dir, "repos", "testrepo")
	filter := NewFileFilter(256 * 1024)
	indexer := NewIndexer(dir, filter, 256*1024)

	// Create initial files and index
	createTestFile(t, repoDir, "main.go", "package main")
	createTestFile(t, repoDir, "deleted.go", "package deleted")
	_, err := indexer.FullIndex("testrepo", repoDir)
	if err != nil {
		t.Fatalf("FullIndex failed: %v", err)
	}

	// Delete file
	if err := os.Remove(filepath.Join(repoDir, "deleted.go")); err != nil {
		t.Fatalf("Failed to remove file: %v", err)
	}

	// Incremental index
	_, err = indexer.IncrementalIndex("testrepo", repoDir, []string{"deleted.go"})
	if err != nil {
		t.Fatalf("IncrementalIndex failed: %v", err)
	}

	// Verify file is removed from index
	docCount, err := indexer.GetDocumentCount("testrepo")
	if err != nil {
		t.Fatalf("GetDocumentCount failed: %v", err)
	}
	if docCount != 1 {
		t.Errorf("Expected 1 document after deletion, got %d", docCount)
	}
}

func TestIndexer_DeleteIndex(t *testing.T) {
	dir := t.TempDir()
	filter := NewFileFilter(256 * 1024)
	indexer := NewIndexer(dir, filter, 256*1024)

	// Create index
	index, err := indexer.OpenForWrite("testrepo")
	if err != nil {
		t.Fatalf("OpenForWrite failed: %v", err)
	}
	closeIndex(t, index)

	if !indexer.IndexExists("testrepo") {
		t.Fatal("Index should exist")
	}

	// Delete index
	if err := indexer.DeleteIndex("testrepo"); err != nil {
		t.Fatalf("DeleteIndex failed: %v", err)
	}

	if indexer.IndexExists("testrepo") {
		t.Error("Index should not exist after deletion")
	}
}

func TestIndexer_GetDocumentCount(t *testing.T) {
	dir := t.TempDir()
	repoDir := filepath.Join(dir, "repos", "testrepo")
	filter := NewFileFilter(256 * 1024)
	indexer := NewIndexer(dir, filter, 256*1024)

	// Create test files
	createTestFile(t, repoDir, "file1.go", "package main")
	createTestFile(t, repoDir, "file2.go", "package other")
	createTestFile(t, repoDir, "file3.go", "package third")

	_, err := indexer.FullIndex("testrepo", repoDir)
	if err != nil {
		t.Fatalf("FullIndex failed: %v", err)
	}

	count, err := indexer.GetDocumentCount("testrepo")
	if err != nil {
		t.Fatalf("GetDocumentCount failed: %v", err)
	}

	if count != 3 {
		t.Errorf("Expected 3 documents, got %d", count)
	}
}

func TestCreateIndexMapping(t *testing.T) {
	mapping := CreateIndexMapping()

	if mapping == nil {
		t.Fatal("Expected non-nil mapping")
	}

	// Verify we can create an index with this mapping
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "test.bleve")

	index, err := bleve.New(indexPath, mapping)
	if err != nil {
		t.Fatalf("Failed to create index with mapping: %v", err)
	}
	defer closeIndex(t, index)
}

// Helper functions

func createTestFile(t *testing.T, baseDir, relPath, content string) {
	t.Helper()
	fullPath := filepath.Join(baseDir, relPath)
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}
}

func createBinaryFile(t *testing.T, baseDir, relPath string) {
	t.Helper()
	fullPath := filepath.Join(baseDir, relPath)
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	// Binary content with null bytes
	content := []byte{'B', 'I', 'N', 0x00, 'A', 'R', 'Y'}
	if err := os.WriteFile(fullPath, content, 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}
}

func makeLargeContent(size int) string {
	content := make([]byte, size)
	for i := range content {
		content[i] = 'x'
	}
	return string(content)
}
