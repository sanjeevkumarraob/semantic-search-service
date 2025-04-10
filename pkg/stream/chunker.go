package stream

import (
	"bytes"
	"errors"
	"io"
)

// ChunkedReader provides chunked reading capability for large files
type ChunkedReader struct {
	reader    io.Reader
	chunkSize int
	buffer    *bytes.Buffer
	eof       bool
}

// NewChunkedReader creates a new chunked reader
func NewChunkedReader(reader io.Reader, chunkSize int) *ChunkedReader {
	return &ChunkedReader{
		reader:    reader,
		chunkSize: chunkSize,
		buffer:    bytes.NewBuffer(make([]byte, 0, chunkSize)),
		eof:       false,
	}
}

// NextChunk reads the next chunk from the reader
func (cr *ChunkedReader) NextChunk() ([]byte, error) {
	// If we've reached EOF in a previous call, return immediately
	if cr.eof && cr.buffer.Len() == 0 {
		return nil, io.EOF
	}
	
	// Reset buffer if it's full
	if cr.buffer.Len() >= cr.chunkSize {
		cr.buffer.Reset()
	}
	
	// Read until we have a full chunk or EOF
	for cr.buffer.Len() < cr.chunkSize && !cr.eof {
		// Prepare temporary buffer for reading
		temp := make([]byte, cr.chunkSize-cr.buffer.Len())
		
		// Read from source
		n, err := cr.reader.Read(temp)
		
		// Handle read result
		if n > 0 {
			cr.buffer.Write(temp[:n])
		}
		
		if err != nil {
			if errors.Is(err, io.EOF) {
				cr.eof = true
				// If we have some data, return it
				if cr.buffer.Len() > 0 {
					return cr.buffer.Bytes(), nil
				}
				// Otherwise, signal EOF
				return nil, io.EOF
			}
			// For other errors, return immediately
			return nil, err
		}
	}
	
	// Return buffer contents
	return cr.buffer.Bytes(), nil
}

// ReadAll reads all chunks into a slice
func (cr *ChunkedReader) ReadAll() ([][]byte, error) {
	var chunks [][]byte
	
	for {
		chunk, err := cr.NextChunk()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		
		// Make a copy of the chunk to avoid overwriting
		chunkCopy := make([]byte, len(chunk))
		copy(chunkCopy, chunk)
		
		chunks = append(chunks, chunkCopy)
	}
	
	return chunks, nil
}