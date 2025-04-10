package atlassian

import (
	"context"
	"fmt"
)

// JiraProject represents a Jira project
type JiraProject struct {
	ID          string `json:"id"`
	Key         string `json:"key"`
	Name        string `json:"name"`
	ProjectType string `json:"projectTypeKey"`
	Style       string `json:"style"`
}

// JiraProjectsResponse represents the Jira projects response
type JiraProjectsResponse struct {
	Values []JiraProject `json:"values"`
}

// JiraIssueType represents a Jira issue type
type JiraIssueType struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Subtask bool   `json:"subtask"`
}

// JiraIssueTypesResponse represents the Jira issue types response
type JiraIssueTypesResponse struct {
	Values []JiraIssueType `json:"values"`
}

// CreateJiraIssueRequest is the request to create a Jira issue
type CreateJiraIssueRequest struct {
	Fields struct {
		Project struct {
			Key string `json:"key"`
		} `json:"project"`
		Summary     string `json:"summary"`
		Description string `json:"description"`
		IssueType   struct {
			ID string `json:"id"`
		} `json:"issuetype"`
	} `json:"fields"`
}

// CreateJiraIssueResponse is the response from creating a Jira issue
type CreateJiraIssueResponse struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Self string `json:"self"`
}

// JiraClient is a client for the Jira API
type JiraClient struct {
	*BaseClient
	cloudID string
}

// NewJiraClient creates a new Jira client
func NewJiraClient(baseURL string) *JiraClient {
	return &JiraClient{
		BaseClient: NewBaseClient(baseURL),
	}
}

// SetCloudID sets the cloud ID for the client
func (c *JiraClient) SetCloudID(cloudID string) {
	c.cloudID = cloudID
}

// ListProjects lists Jira projects
func (c *JiraClient) ListProjects(ctx context.Context, token string) ([]JiraProject, error) {
	if c.cloudID == "" {
		return nil, fmt.Errorf("cloud ID is not set")
	}

	path := fmt.Sprintf("/rest/api/3/project/search?maxResults=100")
	var response JiraProjectsResponse

	err := c.Get(ctx, path, token, &response)
	if err != nil {
		return nil, err
	}

	return response.Values, nil
}

// ListIssueTypes lists Jira issue types
func (c *JiraClient) ListIssueTypes(ctx context.Context, token string) ([]JiraIssueType, error) {
	if c.cloudID == "" {
		return nil, fmt.Errorf("cloud ID is not set")
	}

	path := fmt.Sprintf("/rest/api/3/issuetype")
	var response []JiraIssueType

	err := c.Get(ctx, path, token, &response)
	if err != nil {
		return nil, err
	}

	return response, nil
}

// CreateIssue creates a Jira issue
func (c *JiraClient) CreateIssue(ctx context.Context, token string, projectKey, summary, description, issueTypeID string) (*CreateJiraIssueResponse, error) {
	if c.cloudID == "" {
		return nil, fmt.Errorf("cloud ID is not set")
	}

	path := fmt.Sprintf("/rest/api/3/issue")

	req := CreateJiraIssueRequest{}
	req.Fields.Project.Key = projectKey
	req.Fields.Summary = summary
	req.Fields.Description = description
	req.Fields.IssueType.ID = issueTypeID

	var response CreateJiraIssueResponse

	err := c.Post(ctx, path, token, req, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

// GetIssuePermissions gets permissions for an issue
func (c *JiraClient) GetIssuePermissions(ctx context.Context, token, issueKey string) ([]string, error) {
	// In a real implementation, this would fetch actual permissions
	// For POC, we'll just return the issue key as a permission token
	return []string{issueKey}, nil
}
