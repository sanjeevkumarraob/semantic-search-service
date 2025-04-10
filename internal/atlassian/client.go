package atlassian

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// BaseClient is the base client for Atlassian APIs
type BaseClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewBaseClient creates a new base client
func NewBaseClient(baseURL string) *BaseClient {
	return &BaseClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Get performs a GET request
func (c *BaseClient) Get(ctx context.Context, path string, token string, result interface{}) error {
	return c.request(ctx, http.MethodGet, path, token, nil, result)
}

// Post performs a POST request
func (c *BaseClient) Post(ctx context.Context, path string, token string, body, result interface{}) error {
	return c.request(ctx, http.MethodPost, path, token, body, result)
}

// Put performs a PUT request
func (c *BaseClient) Put(ctx context.Context, path string, token string, body, result interface{}) error {
	return c.request(ctx, http.MethodPut, path, token, body, result)
}

// Delete performs a DELETE request
func (c *BaseClient) Delete(ctx context.Context, path string, token string) error {
	return c.request(ctx, http.MethodDelete, path, token, nil, nil)
}

// request performs an HTTP request
func (c *BaseClient) request(ctx context.Context, method, path, token string, body, result interface{}) error {
	url := fmt.Sprintf("%s%s", c.baseURL, path)

	// Debug logging
	fmt.Printf("Making %s request to: %s\n", method, url)

	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
		// Debug token (truncated for security)
		if len(token) > 10 {
			fmt.Printf("Using token: %s...[truncated for security]\n", token[:10])
		}
	}

	// Debug request headers
	fmt.Printf("Request headers: %v\n", req.Header)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body for debugging
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Debug response
	fmt.Printf("Response status: %d %s\n", resp.StatusCode, resp.Status)
	fmt.Printf("Response headers: %v\n", resp.Header)

	// For debugging, show the response body (with some limits)
	if len(respBody) > 500 {
		fmt.Printf("Response body (truncated): %s...\n", respBody[:500])
	} else if len(respBody) > 0 {
		fmt.Printf("Response body: %s\n", respBody)
	}

	// Reset the response body for further processing
	resp.Body = io.NopCloser(bytes.NewReader(respBody))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("failed to decode response: %w, body: %s", err, string(respBody))
		}
	}

	return nil
}

// GetBaseURL returns the base URL for the client
func (c *BaseClient) GetBaseURL() string {
	return c.baseURL
}
