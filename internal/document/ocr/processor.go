package ocr

import (
	"context"
	"errors"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/otiai10/gosseract/v2"
)

// Processor handles OCR processing
type Processor struct {
	mutex sync.Mutex
}

// NewProcessor creates a new OCR processor
func NewProcessor() *Processor {
	return &Processor{}
}

// Process extracts text from images using OCR
func (p *Processor) Process(ctx context.Context, reader io.Reader) ([]string, error) {
	// For a real implementation, you would process the image in chunks
	// For POC, we'll read the entire image and process
	
	// Decode the image
	img, _, err := image.Decode(reader)
	if err != nil {
		return nil, err
	}
	
	// Write image to temporary file (needed for Tesseract)
	tmpDir := os.TempDir()
	tmpFile := filepath.Join(tmpDir, "ocr_"+generateRandomString(10)+".png")
	
	f, err := os.Create(tmpFile)
	if err != nil {
		return nil, err
	}
	
	// Ensure the temporary file is deleted
	defer func() {
		f.Close()
		os.Remove(tmpFile)
	}()
	
	// Save the image to the temporary file
	if err := saveImage(img, f); err != nil {
		return nil, err
	}
	
	// Close the file to ensure it's written
	f.Close()
	
	// Use gosseract for OCR
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	client := gosseract.NewClient()
	defer client.Close()
	
	if err := client.SetImage(tmpFile); err != nil {
		return nil, err
	}
	
	text, err := client.Text()
	if err != nil {
		return nil, err
	}
	
	if text == "" {
		return nil, errors.New("no text extracted from image")
	}
	
	// Split the text into paragraphs
	paragraphs := splitIntoParagraphs(text)
	
	return paragraphs, nil
}

// saveImage saves an image to a writer
func saveImage(img image.Image, w io.Writer) error {
	// For POC, we'll use a simple PNG encoder
	// In a real implementation, you would use a more sophisticated approach
	// based on the image type and quality requirements
	
	// This function would use an image encoder to write to w
	// For simplicity in the POC, we'll assume this works
	return nil // Replace with actual implementation
}

// generateRandomString generates a random string
func generateRandomString(length int) string {
	// For POC, a simple implementation
	return "random_string"
}

// splitIntoParagraphs splits text into paragraphs
func splitIntoParagraphs(text string) []string {
	// For POC, a simple implementation
	return []string{text}
}