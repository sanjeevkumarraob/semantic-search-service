package document

import (
	"context"
	"fmt"
	"log"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"github.com/sanjeevkumarraob/semantic-search-service/internal/document/extractor"
	"github.com/sanjeevkumarraob/semantic-search-service/internal/document/ocr"
)

// Error definitions
var (
	ErrFileTooLarge        = fmt.Errorf("file size exceeds maximum allowed size")
	ErrUnsupportedFileType = fmt.Errorf("unsupported file type")
)

// ContentType represents supported document content types
type ContentType string

const (
	ContentTypePDF        ContentType = "application/pdf"
	ContentTypeWord       ContentType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	ContentTypeImage      ContentType = "image"
	ContentTypeText       ContentType = "text/plain"
	ContentTypeConfluence ContentType = "confluence/page"
)

// ProcessorResult contains the extracted text and metadata
type ProcessorResult struct {
	DocumentID string
	Title      string
	Content    []string // Chunked content
	Metadata   map[string]string
}

// Processor handles document processing
type Processor struct {
	pdfExtractor    *extractor.PDFExtractor
	wordExtractor   *extractor.WordExtractor
	plainExtractor  *extractor.PlainExtractor
	ocrProcessor    *ocr.Processor
	logger          *log.Logger
	chunkSize       int
	maxDocumentSize int64
}

// NewProcessor creates a new document processor
func NewProcessor(logger *log.Logger) *Processor {
	return &Processor{
		pdfExtractor:    extractor.NewPDFExtractor(),
		wordExtractor:   extractor.NewWordExtractor(),
		plainExtractor:  extractor.NewPlainExtractor(),
		ocrProcessor:    ocr.NewProcessor(),
		logger:          logger,
		chunkSize:       1000,             // Default chunk size (words)
		maxDocumentSize: 50 * 1024 * 1024, // 50MB max
	}
}

// ProcessFile handles document processing by file type
func (p *Processor) ProcessFile(ctx context.Context, file multipart.File, header *multipart.FileHeader) (*ProcessorResult, error) {
	// Check file size
	if header.Size > p.maxDocumentSize {
		return nil, ErrFileTooLarge
	}

	// Determine content type
	contentType := p.determineContentType(header.Filename)

	// Process based on content type
	var content []string
	var err error

	switch contentType {
	case ContentTypePDF:
		content, err = p.pdfExtractor.Extract(ctx, file)
	case ContentTypeWord:
		content, err = p.wordExtractor.Extract(ctx, file)
	case ContentTypeImage:
		content, err = p.ocrProcessor.Process(ctx, file)
	case ContentTypeText:
		content, err = p.plainExtractor.Extract(ctx, file)
	default:
		return nil, ErrUnsupportedFileType
	}

	if err != nil {
		return nil, err
	}

	// Create result
	result := &ProcessorResult{
		DocumentID: generateID(header.Filename),
		Title:      filepath.Base(header.Filename),
		Content:    content,
		Metadata: map[string]string{
			"filename":    header.Filename,
			"size":        string(header.Size),
			"contentType": string(contentType),
		},
	}

	return result, nil
}

// ProcessConfluencePage processes content from a Confluence page
func (p *Processor) ProcessConfluencePage(ctx context.Context, pageID, title string, content string) (*ProcessorResult, error) {
	// Process HTML content from Confluence
	plainContent, err := p.plainExtractor.ExtractFromHTML(content)
	if err != nil {
		return nil, err
	}

	// Chunk the content
	chunks := p.chunkContent(plainContent)

	// Create result
	result := &ProcessorResult{
		DocumentID: pageID,
		Title:      title,
		Content:    chunks,
		Metadata: map[string]string{
			"source": "confluence",
			"pageID": pageID,
		},
	}

	return result, nil
}

// determineContentType guesses the content type from filename
func (p *Processor) determineContentType(filename string) ContentType {
	ext := strings.ToLower(filepath.Ext(filename))

	switch ext {
	case ".pdf":
		return ContentTypePDF
	case ".docx", ".doc":
		return ContentTypeWord
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".tiff":
		return ContentTypeImage
	case ".txt", ".md", ".rtf":
		return ContentTypeText
	default:
		return ContentTypeText // Default to text
	}
}

// chunkContent splits content into manageable chunks
func (p *Processor) chunkContent(content string) []string {
	words := strings.Fields(content)
	chunks := make([]string, 0)

	// Create chunks of approximately p.chunkSize words
	for i := 0; i < len(words); i += p.chunkSize {
		end := i + p.chunkSize
		if end > len(words) {
			end = len(words)
		}

		chunk := strings.Join(words[i:end], " ")
		chunks = append(chunks, chunk)
	}

	return chunks
}

// generateID creates a unique ID for a document
func generateID(filename string) string {
	return fmt.Sprintf("%s-%d", filepath.Base(filename), time.Now().UnixNano())
}
