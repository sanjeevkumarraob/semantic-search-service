package extractor

import (
	"bufio"
	"context"
	"errors"
	"io"
	"strings"

	"golang.org/x/net/html"
)

// PlainExtractor extracts text from plain text documents
type PlainExtractor struct{}

// NewPlainExtractor creates a new plain text extractor
func NewPlainExtractor() *PlainExtractor {
	return &PlainExtractor{}
}

// Extract extracts text from a plain text document
func (e *PlainExtractor) Extract(ctx context.Context, reader io.Reader) ([]string, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Split(bufio.ScanLines)
	
	var extractedText []string
	var currentParagraph strings.Builder
	
	for scanner.Scan() {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		
		line := scanner.Text()
		
		// If line is empty, save the current paragraph and start a new one
		if strings.TrimSpace(line) == "" {
			if currentParagraph.Len() > 0 {
				extractedText = append(extractedText, currentParagraph.String())
				currentParagraph.Reset()
			}
		} else {
			// Add the line to current paragraph
			if currentParagraph.Len() > 0 {
				currentParagraph.WriteString(" ")
			}
			currentParagraph.WriteString(line)
		}
	}
	
	// Save the last paragraph if not empty
	if currentParagraph.Len() > 0 {
		extractedText = append(extractedText, currentParagraph.String())
	}
	
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	
	if len(extractedText) == 0 {
		return nil, errors.New("no text extracted from document")
	}
	
	return extractedText, nil
}

// ExtractFromHTML extracts text from HTML content
func (e *PlainExtractor) ExtractFromHTML(htmlContent string) (string, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return "", err
	}
	
	var textBuilder strings.Builder
	e.extractTextFromNode(doc, &textBuilder)
	
	return textBuilder.String(), nil
}

// extractTextFromNode recursively extracts text from HTML nodes
func (e *PlainExtractor) extractTextFromNode(n *html.Node, textBuilder *strings.Builder) {
	if n.Type == html.TextNode {
		text := strings.TrimSpace(n.Data)
		if text != "" {
			textBuilder.WriteString(text + " ")
		}
	}
	
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		e.extractTextFromNode(c, textBuilder)
	}
}