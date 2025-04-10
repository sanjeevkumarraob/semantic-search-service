package search

import (
	"context"
	"log"
	"time"

	"github.com/sanjeevkumarraob/semantic-search-service/internal/document"
	"github.com/sanjeevkumarraob/semantic-search-service/pkg/vectorstore"
)

// SearchRequest represents a search query
type SearchRequest struct {
	Query       string
	UserID      string
	Permissions []string // List of content IDs the user has access to
	Limit       int
}

// SearchResult represents a search result
type SearchResult struct {
	DocumentID   string
	Title        string
	ChunkContent string
	Score        float64
	Metadata     map[string]string
}

// Engine handles search operations
type Engine struct {
	embedder    *Embedder
	vectorStore *vectorstore.QdrantStore
	ttl         time.Duration
	logger      *log.Logger
}

// NewEngine creates a new search engine
func NewEngine(logger *log.Logger) *Engine {
	// Initialize embedder
	embedder := NewEmbedder()

	// Initialize vector store with in-memory configuration
	vectorStore := vectorstore.NewQdrantStore(&vectorstore.Config{
		InMemory:   true,
		VectorSize: embedder.VectorSize(),
		TTL:        30 * time.Minute,
	})

	return &Engine{
		embedder:    embedder,
		vectorStore: vectorStore,
		ttl:         30 * time.Minute, // Default TTL for vectors
		logger:      logger,
	}
}

// IndexDocument processes and indexes document content
func (e *Engine) IndexDocument(ctx context.Context, doc *document.ProcessorResult, userPermissions []string) error {
	// Process each content chunk
	for i, chunk := range doc.Content {
		// Generate embedding for this chunk
		embedding, err := e.embedder.Embed(ctx, chunk)
		if err != nil {
			e.logger.Printf("Error embedding chunk %d of document %s: %v", i, doc.DocumentID, err)
			continue
		}

		// Create a unique ID for this chunk
		chunkID := doc.DocumentID + "-" + string(i)

		// Store vector with permissions as payload
		err = e.vectorStore.Store(ctx, &vectorstore.Item{
			ID:         chunkID,
			Vector:     embedding,
			DocumentID: doc.DocumentID,
			Content:    chunk,
			Title:      doc.Title,
			Metadata:   doc.Metadata,
			// Store permissions with the vector for filtering
			Permissions: userPermissions,
			// Set expiration time
			ExpiresAt: time.Now().Add(e.ttl),
		})

		if err != nil {
			e.logger.Printf("Failed to store vector for chunk %d of document %s: %v", i, doc.DocumentID, err)
			return err
		}
	}

	return nil
}

// Search performs semantic search
func (e *Engine) Search(ctx context.Context, req *SearchRequest) ([]SearchResult, error) {
	// Generate embedding for query
	queryEmbedding, err := e.embedder.Embed(ctx, req.Query)
	if err != nil {
		return nil, err
	}

	// Limit defaults
	if req.Limit <= 0 {
		req.Limit = 10
	}

	// Search vectors, filtering by user permissions
	results, err := e.vectorStore.Search(ctx, &vectorstore.SearchParams{
		Vector:           queryEmbedding,
		Limit:            req.Limit,
		PermissionFilter: req.Permissions,
	})

	if err != nil {
		return nil, err
	}

	// Convert to search results
	searchResults := make([]SearchResult, len(results))
	for i, result := range results {
		searchResults[i] = SearchResult{
			DocumentID:   result.DocumentID,
			Title:        result.Title,
			ChunkContent: result.Content,
			Score:        result.Score,
			Metadata:     result.Metadata,
		}
	}

	return searchResults, nil
}

// Cleanup performs necessary cleanup operations
func (e *Engine) Cleanup() {
	e.vectorStore.Close()
}
