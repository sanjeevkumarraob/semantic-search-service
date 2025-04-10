package extractor

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"

	"github.com/unidoc/unioffice/document"
)

// WordExtractor extracts text from Word documents
type WordExtractor struct{}

// NewWordExtractor creates a new Word extractor
func NewWordExtractor() *WordExtractor {
	return &WordExtractor{}
}

// Extract extracts text from a Word document
func (e *WordExtractor) Extract(ctx context.Context, reader io.Reader) ([]string, error) {
	// For a real implementation, you would use a streaming approach
	// For POC, we'll read the entire file and process

	// Read the document into memory
	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	// Use UniOffice to extract text
	doc, err := document.Read(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return nil, err
	}

	var extractedText []string

	// Process each paragraph
	for _, para := range doc.Paragraphs() {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		var paraText strings.Builder

		// Process each run (text block) in the paragraph
		for _, run := range para.Runs() {
			paraText.WriteString(run.Text())
		}

		text := strings.TrimSpace(paraText.String())
		if text != "" {
			extractedText = append(extractedText, text)
		}
	}

	if len(extractedText) == 0 {
		return nil, errors.New("no text extracted from Word document")
	}

	return extractedText, nil
}
