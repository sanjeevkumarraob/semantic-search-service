package search

import (
	"context"
	"math"
	"math/rand"
)

// Embedder generates vector embeddings from text
type Embedder struct {
	vectorSize int
}

// NewEmbedder creates a new embedder instance
func NewEmbedder() *Embedder {
	// For POC, we'll use a simple random embedding approach
	// In production, you would use a proper embedding model

	return &Embedder{
		vectorSize: 384, // Standard embedding size
	}
}

// Embed generates a vector embedding for the given text
func (e *Embedder) Embed(ctx context.Context, text string) ([]float32, error) {
	// Check context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// For POC, we'll generate random embeddings based on the text
	// This is a placeholder for a real embedding model
	embedding := make([]float32, e.vectorSize)

	// Use the text as a seed for reproducibility
	seed := int64(0)
	for _, c := range text {
		seed = seed*31 + int64(c)
	}

	r := rand.New(rand.NewSource(seed))

	// Generate random values
	for i := 0; i < e.vectorSize; i++ {
		embedding[i] = float32(r.NormFloat64())
	}

	// Normalize the vector
	var sum float64
	for _, v := range embedding {
		sum += float64(v * v)
	}

	norm := float32(math.Sqrt(sum))
	if norm > 0 {
		for i := 0; i < e.vectorSize; i++ {
			embedding[i] /= norm
		}
	}

	return embedding, nil
}

// VectorSize returns the dimensionality of the embeddings
func (e *Embedder) VectorSize() int {
	return e.vectorSize
}

// Close releases resources
func (e *Embedder) Close() {
	// No resources to release
}
