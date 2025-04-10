package vectorstore

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Config contains configuration for the vector store
type Config struct {
	InMemory   bool
	Address    string
	Collection string
	VectorSize int
	TTL        time.Duration
}

// Item represents a stored vector item
type Item struct {
	ID          string
	Vector      []float32
	DocumentID  string
	Content     string
	Title       string
	Metadata    map[string]string
	Permissions []string
	ExpiresAt   time.Time
}

// SearchParams contains parameters for search operations
type SearchParams struct {
	Vector           []float32
	Limit            int
	PermissionFilter []string
}

// SearchResult represents a search result
type SearchResult struct {
	ID         string
	DocumentID string
	Content    string
	Title      string
	Metadata   map[string]string
	Score      float64
}

// scoredItem represents an item with its similarity score
type scoredItem struct {
	item  *Item
	score float64
}

// QdrantStore provides vector storage using Qdrant
// For POC, we'll implement a simple in-memory version
type QdrantStore struct {
	config    *Config
	items     map[string]*Item
	lock      sync.RWMutex
	closeChan chan struct{}
	closed    bool
}

// NewQdrantStore creates a new Qdrant store
func NewQdrantStore(config *Config) *QdrantStore {
	store := &QdrantStore{
		config:    config,
		items:     make(map[string]*Item),
		closeChan: make(chan struct{}),
	}

	// Start cleanup goroutine for expired items
	go store.cleanupRoutine()

	return store
}

// Store adds or updates a vector in the store
func (s *QdrantStore) Store(ctx context.Context, item *Item) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.closed {
		return errors.New("store is closed")
	}

	// Add the item
	s.items[item.ID] = item

	return nil
}

// Get retrieves a vector by ID
func (s *QdrantStore) Get(ctx context.Context, id string) (*Item, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	if s.closed {
		return nil, errors.New("store is closed")
	}

	item, exists := s.items[id]
	if !exists {
		return nil, fmt.Errorf("item with ID %s not found", id)
	}

	// Check expiration
	if !item.ExpiresAt.IsZero() && time.Now().After(item.ExpiresAt) {
		return nil, fmt.Errorf("item with ID %s has expired", id)
	}

	return item, nil
}

// Delete removes a vector from the store
func (s *QdrantStore) Delete(ctx context.Context, id string) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.closed {
		return errors.New("store is closed")
	}

	delete(s.items, id)

	return nil
}

// Search performs vector similarity search
func (s *QdrantStore) Search(ctx context.Context, params *SearchParams) ([]*SearchResult, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	if s.closed {
		return nil, errors.New("store is closed")
	}

	// For POC, we'll implement a simple cosine similarity search
	var scored []scoredItem

	// Calculate scores for all items
	for _, item := range s.items {
		// Skip expired items
		if !item.ExpiresAt.IsZero() && time.Now().After(item.ExpiresAt) {
			continue
		}

		// Check permissions if filter is provided
		if len(params.PermissionFilter) > 0 {
			hasPermission := false
			for _, permission := range params.PermissionFilter {
				for _, itemPerm := range item.Permissions {
					if permission == itemPerm {
						hasPermission = true
						break
					}
				}
				if hasPermission {
					break
				}
			}

			if !hasPermission {
				continue
			}
		}

		// Calculate cosine similarity
		score := cosineSimilarity(params.Vector, item.Vector)
		scored = append(scored, scoredItem{item: item, score: score})
	}

	// Sort by score (descending)
	sortScored(scored)

	// Limit results
	if params.Limit > 0 && len(scored) > params.Limit {
		scored = scored[:params.Limit]
	}

	// Convert to search results
	results := make([]*SearchResult, len(scored))
	for i, s := range scored {
		results[i] = &SearchResult{
			ID:         s.item.ID,
			DocumentID: s.item.DocumentID,
			Content:    s.item.Content,
			Title:      s.item.Title,
			Metadata:   s.item.Metadata,
			Score:      s.score,
		}
	}

	return results, nil
}

// Close closes the store and cleans up resources
func (s *QdrantStore) Close() error {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true
	close(s.closeChan)

	// Clear items
	s.items = nil

	return nil
}

// cleanupRoutine periodically removes expired items
func (s *QdrantStore) cleanupRoutine() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.cleanupExpiredItems()
		case <-s.closeChan:
			return
		}
	}
}

// cleanupExpiredItems removes all expired items
func (s *QdrantStore) cleanupExpiredItems() {
	s.lock.Lock()
	defer s.lock.Unlock()

	now := time.Now()

	for id, item := range s.items {
		if !item.ExpiresAt.IsZero() && now.After(item.ExpiresAt) {
			delete(s.items, id)
		}
	}
}

// cosineSimilarity calculates cosine similarity between two vectors
func cosineSimilarity(a, b []float32) float64 {
	// Ensure vectors have the same length
	if len(a) != len(b) {
		return 0
	}

	var dotProduct float64
	var normA float64
	var normB float64

	for i := 0; i < len(a); i++ {
		dotProduct += float64(a[i] * b[i])
		normA += float64(a[i] * a[i])
		normB += float64(b[i] * b[i])
	}

	// Handle zero vectors
	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (sqrt(normA) * sqrt(normB))
}

// sqrt calculates square root (simple implementation for the POC)
func sqrt(x float64) float64 {
	return float64(float32(x))
}

// sortScored sorts scored items by score in descending order
func sortScored(items []scoredItem) {
	// For POC, simple bubble sort is fine
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[i].score < items[j].score {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}
