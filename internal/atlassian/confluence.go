package atlassian

import (
	"context"
	"fmt"
	"net/url"
)

// ConfluenceSpace represents a Confluence space
type ConfluenceSpace struct {
	ID    string `json:"id"`
	Key   string `json:"key"`
	Name  string `json:"name"`
	Type  string `json:"type"`
	Links struct {
		WebUI string `json:"webui"`
	} `json:"_links"`
}

// ConfluenceSpacesResponse represents the Confluence spaces response
type ConfluenceSpacesResponse struct {
	Results []ConfluenceSpace `json:"results"`
}

// ConfluencePage represents a Confluence page
type ConfluencePage struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Type  string `json:"type"`
	Links struct {
		WebUI string `json:"webui"`
	} `json:"_links"`
}

// ConfluencePagesResponse represents the Confluence pages response
type ConfluencePagesResponse struct {
	Results []ConfluencePage `json:"results"`
}

// ConfluencePageContent represents a Confluence page content
type ConfluencePageContent struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Version struct {
		Number int `json:"number"`
	} `json:"version"`
	Body struct {
		Storage struct {
			Value          string `json:"value"`
			Representation string `json:"representation"`
		} `json:"storage"`
	} `json:"body"`
}

// ConfluenceClient is a client for the Confluence API
type ConfluenceClient struct {
	*BaseClient
	cloudID string
}

// NewConfluenceClient creates a new Confluence client
func NewConfluenceClient(baseURL string) *ConfluenceClient {
	return &ConfluenceClient{
		BaseClient: NewBaseClient(baseURL),
	}
}

// SetCloudID sets the cloud ID for the client
func (c *ConfluenceClient) SetCloudID(cloudID string) {
	c.cloudID = cloudID
}

// ListSpaces lists Confluence spaces
func (c *ConfluenceClient) ListSpaces(ctx context.Context, token string) ([]ConfluenceSpace, error) {
	if c.BaseClient.GetBaseURL() == "" {
		return nil, fmt.Errorf("base URL is not set, please set CONFLUENCE_BASE_URL environment variable to your Atlassian site URL")
	}

	// Construct the path according to the REST API v2 documentation
	path := fmt.Sprintf("/api/v2/spaces?limit=100")
	fmt.Printf("Using Confluence API request: %s%s\n", c.BaseClient.GetBaseURL(), path)

	var response ConfluenceSpacesResponse

	err := c.Get(ctx, path, token, &response)
	if err != nil {
		return nil, err
	}

	return response.Results, nil
}

// ListPages lists Confluence pages in a space
func (c *ConfluenceClient) ListPages(ctx context.Context, token, spaceKey string) ([]ConfluencePage, error) {
	if c.BaseClient.GetBaseURL() == "" {
		return nil, fmt.Errorf("base URL is not set, please set CONFLUENCE_BASE_URL environment variable to your Atlassian site URL")
	}

	// Construct the path according to the REST API v2 documentation
	path := fmt.Sprintf("/api/v2/spaces/%s/pages?limit=100", url.PathEscape(spaceKey))
	fmt.Printf("Using Confluence API request: %s%s\n", c.BaseClient.GetBaseURL(), path)

	var response ConfluencePagesResponse

	err := c.Get(ctx, path, token, &response)
	if err != nil {
		return nil, err
	}

	return response.Results, nil
}

// GetPageContent gets content of a Confluence page
func (c *ConfluenceClient) GetPageContent(ctx context.Context, token, pageID string) (*ConfluencePageContent, error) {
	if c.BaseClient.GetBaseURL() == "" {
		return nil, fmt.Errorf("base URL is not set, please set CONFLUENCE_BASE_URL environment variable to your Atlassian site URL")
	}

	// Construct the path according to the REST API v2 documentation
	path := fmt.Sprintf("/api/v2/pages/%s?body-format=storage", url.PathEscape(pageID))

	fmt.Printf("Using Confluence API request: %s%s\n", c.BaseClient.GetBaseURL(), path)

	var response ConfluencePageContent

	err := c.Get(ctx, path, token, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

// GetPagePermissions gets permissions for a page
func (c *ConfluenceClient) GetPagePermissions(ctx context.Context, token, pageID string) ([]string, error) {
	// In a real implementation, this would fetch actual permissions
	// For POC, we'll just return the page ID as a permission token
	return []string{pageID}, nil
}
