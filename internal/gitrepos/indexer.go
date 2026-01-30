package gitrepos

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/analysis/analyzer/keyword"
	"github.com/blevesearch/bleve/v2/analysis/analyzer/standard"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/sha1n/mcp-relic-server/internal/domain"
)

const (
	// IndexSuffix is the suffix for index directories
	IndexSuffix = ".bleve"

	// MaxBatchSize is the maximum number of documents per batch
	MaxBatchSize = 100

	// MaxBatchBytes is the maximum bytes per batch (10MB)
	MaxBatchBytes = 10 * 1024 * 1024
)

// Indexer manages Bleve indexes for repositories.
type Indexer struct {
	baseDir     string
	filter      *FileFilter
	maxFileSize int64
}

// NewIndexer creates a new indexer.
func NewIndexer(baseDir string, filter *FileFilter, maxFileSize int64) *Indexer {
	return &Indexer{
		baseDir:     baseDir,
		filter:      filter,
		maxFileSize: maxFileSize,
	}
}

// indexPath returns the path to an index for a given repo ID.
func (i *Indexer) indexPath(repoID string) string {
	return filepath.Join(i.baseDir, "indexes", repoID+IndexSuffix)
}

// CreateIndexMapping creates the Bleve index mapping for code documents.
func CreateIndexMapping() mapping.IndexMapping {
	// Create document mapping for CodeDocument
	docMapping := bleve.NewDocumentMapping()

	// Content field - analyzed for full-text search
	contentField := bleve.NewTextFieldMapping()
	contentField.Analyzer = standard.Name
	contentField.Store = true
	contentField.IncludeTermVectors = true
	docMapping.AddFieldMappingsAt(domain.CodeFieldContent, contentField)

	// Repository - keyword (not analyzed), stored for retrieval
	repoField := bleve.NewTextFieldMapping()
	repoField.Analyzer = keyword.Name
	repoField.Store = true
	docMapping.AddFieldMappingsAt(domain.CodeFieldRepository, repoField)

	// Extension - keyword, stored
	extField := bleve.NewTextFieldMapping()
	extField.Analyzer = keyword.Name
	extField.Store = true
	docMapping.AddFieldMappingsAt(domain.CodeFieldExtension, extField)

	// FilePath - keyword, stored
	pathField := bleve.NewTextFieldMapping()
	pathField.Analyzer = keyword.Name
	pathField.Store = true
	docMapping.AddFieldMappingsAt(domain.CodeFieldFilePath, pathField)

	// ID - stored but not indexed (we use the document ID)
	idField := bleve.NewTextFieldMapping()
	idField.Index = false
	idField.Store = true
	docMapping.AddFieldMappingsAt(domain.CodeFieldID, idField)

	// Create the index mapping
	indexMapping := bleve.NewIndexMapping()
	indexMapping.DefaultMapping = docMapping
	indexMapping.DefaultAnalyzer = standard.Name

	return indexMapping
}

// OpenForWrite opens or creates an index for writing.
func (i *Indexer) OpenForWrite(repoID string) (bleve.Index, error) {
	indexPath := i.indexPath(repoID)

	// Try to open existing index
	index, err := bleve.Open(indexPath)
	if err == nil {
		return index, nil
	}

	// Create new index
	indexMapping := CreateIndexMapping()
	index, err = bleve.New(indexPath, indexMapping)
	if err != nil {
		return nil, fmt.Errorf("failed to create index: %w", err)
	}

	return index, nil
}

// OpenForRead opens an existing index for reading.
func (i *Indexer) OpenForRead(repoID string) (bleve.Index, error) {
	indexPath := i.indexPath(repoID)

	index, err := bleve.Open(indexPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open index: %w", err)
	}

	return index, nil
}

// IndexExists checks if an index exists for the given repo ID.
func (i *Indexer) IndexExists(repoID string) bool {
	indexPath := i.indexPath(repoID)
	_, err := os.Stat(indexPath)
	return err == nil
}

// CreateAlias creates an IndexAlias combining multiple indexes.
func (i *Indexer) CreateAlias(repoIDs []string) (bleve.IndexAlias, error) {
	indexes := make([]bleve.Index, 0, len(repoIDs))

	for _, repoID := range repoIDs {
		index, err := i.OpenForRead(repoID)
		if err != nil {
			// Close already opened indexes
			for _, idx := range indexes {
				_ = idx.Close()
			}
			return nil, fmt.Errorf("failed to open index for %s: %w", repoID, err)
		}
		indexes = append(indexes, index)
	}

	if len(indexes) == 0 {
		return nil, fmt.Errorf("no indexes to combine")
	}

	return bleve.NewIndexAlias(indexes...), nil
}

// FullIndex performs a full index of a repository.
// Returns the number of files indexed.
func (i *Indexer) FullIndex(repoID, repoDir string) (count int, err error) {
	index, err := i.OpenForWrite(repoID)
	if err != nil {
		return 0, err
	}
	defer func() {
		if cerr := index.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	batch := index.NewBatch()
	batchSize := 0
	batchBytes := 0
	totalIndexed := 0
	displayName := RepoIDToDisplay(repoID)

	err = filepath.WalkDir(repoDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip files with errors
		}

		// Get relative path
		relPath, err := filepath.Rel(repoDir, path)
		if err != nil {
			return nil
		}

		// Skip directories
		if d.IsDir() {
			// Skip .git directory entirely
			if relPath == ".git" || strings.HasPrefix(relPath, ".git/") {
				return filepath.SkipDir
			}
			return nil
		}

		// Check exclusion patterns
		if i.filter.ShouldExclude(relPath) {
			return nil
		}

		// Check file size
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if info.Size() > i.maxFileSize {
			return nil
		}

		// Read file content
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		// Skip binary files
		if IsBinary(content) {
			return nil
		}

		// Create document
		doc := domain.CodeDocument{
			ID:         repoID + "/" + relPath,
			Repository: displayName,
			FilePath:   relPath,
			Extension:  GetFileExtension(relPath),
			Content:    string(content),
		}

		// Add to batch
		if err := batch.Index(doc.ID, doc); err != nil {
			return nil // Skip on indexing error
		}
		batchSize++
		batchBytes += len(content)

		// Flush batch if needed
		if batchSize >= MaxBatchSize || batchBytes >= MaxBatchBytes {
			if err := index.Batch(batch); err != nil {
				return fmt.Errorf("batch index failed: %w", err)
			}
			totalIndexed += batchSize
			batch = index.NewBatch()
			batchSize = 0
			batchBytes = 0
		}

		return nil
	})

	if err != nil {
		return totalIndexed, err
	}

	// Flush remaining batch
	if batchSize > 0 {
		if err := index.Batch(batch); err != nil {
			return totalIndexed, fmt.Errorf("final batch index failed: %w", err)
		}
		totalIndexed += batchSize
	}

	return totalIndexed, nil
}

// IncrementalIndex updates the index for changed files only.
func (i *Indexer) IncrementalIndex(repoID, repoDir string, changedFiles []string) (indexed int, err error) {
	index, err := i.OpenForWrite(repoID)
	if err != nil {
		return 0, err
	}
	defer func() {
		if cerr := index.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	batch := index.NewBatch()
	displayName := RepoIDToDisplay(repoID)

	for _, relPath := range changedFiles {
		fullPath := filepath.Join(repoDir, relPath)
		docID := repoID + "/" + relPath

		// Check if file exists
		info, err := os.Stat(fullPath)
		if os.IsNotExist(err) {
			// File was deleted, remove from index
			batch.Delete(docID)
			continue
		}
		if err != nil {
			continue // Skip on error
		}

		// Skip directories
		if info.IsDir() {
			continue
		}

		// Check exclusion patterns
		if i.filter.ShouldExclude(relPath) {
			// Remove from index in case it was previously indexed
			batch.Delete(docID)
			continue
		}

		// Check file size
		if info.Size() > i.maxFileSize {
			batch.Delete(docID)
			continue
		}

		// Read file content
		content, err := os.ReadFile(fullPath)
		if err != nil {
			continue // Skip on error
		}

		// Skip binary files
		if IsBinary(content) {
			batch.Delete(docID)
			continue
		}

		// Create document
		doc := domain.CodeDocument{
			ID:         docID,
			Repository: displayName,
			FilePath:   relPath,
			Extension:  GetFileExtension(relPath),
			Content:    string(content),
		}

		if err := batch.Index(doc.ID, doc); err != nil {
			continue
		}
		indexed++
	}

	if err := index.Batch(batch); err != nil {
		return indexed, fmt.Errorf("batch index failed: %w", err)
	}

	return indexed, nil
}

// DeleteIndex removes an index from disk.
func (i *Indexer) DeleteIndex(repoID string) error {
	indexPath := i.indexPath(repoID)
	return os.RemoveAll(indexPath)
}

// GetDocumentCount returns the number of documents in an index.
func (i *Indexer) GetDocumentCount(repoID string) (count uint64, err error) {
	index, err := i.OpenForRead(repoID)
	if err != nil {
		return 0, err
	}
	defer func() {
		if cerr := index.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	return index.DocCount()
}
