package extractor

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"

	"github.com/ledongthuc/pdf"
)

// PDFExtractor extracts text from PDF documents
type PDFExtractor struct{}

// NewPDFExtractor creates a new PDF extractor
func NewPDFExtractor() *PDFExtractor {
	return &PDFExtractor{}
}

// Extract extracts text from a PDF document
func (e *PDFExtractor) Extract(ctx context.Context, reader io.Reader) ([]string, error) {
	// For a real implementation, you would use a streaming approach
	// For POC, we'll read the entire file and process

	// Read the PDF into a temp file
	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	// Create PDF reader
	r, err := pdf.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return nil, err
	}

	// Get number of pages
	numPages := r.NumPage()

	// Extract text from each page
	var extractedText []string

	for i := 1; i <= numPages; i++ {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		p := r.Page(i)
		if p.V.IsNull() {
			continue
		}

		text, err := p.GetPlainText(nil)
		if err != nil {
			continue
		}

		// Add text to results if not empty
		if text = strings.TrimSpace(text); text != "" {
			extractedText = append(extractedText, text)
		}
	}

	if len(extractedText) == 0 {
		return nil, errors.New("no text extracted from PDF")
	}

	return extractedText, nil
}
