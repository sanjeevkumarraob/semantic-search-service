package api

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/sanjeevkumarraob/semantic-search-service/internal/atlassian"
	"github.com/sanjeevkumarraob/semantic-search-service/internal/auth"
	"github.com/sanjeevkumarraob/semantic-search-service/internal/document"
	"github.com/sanjeevkumarraob/semantic-search-service/internal/search"
	"github.com/sanjeevkumarraob/semantic-search-service/internal/session"
)

// Handler handles API requests
type Handler struct {
	atlassianAuth    *auth.AtlassianAuth
	confluenceClient *atlassian.ConfluenceClient
	jiraClient       *atlassian.JiraClient
	docProcessor     *document.Processor
	searchEngine     *search.Engine
	logger           *log.Logger
	sessionManager   *session.SessionManager
}

// NewHandler creates a new handler
func NewHandler(
	atlassianAuth *auth.AtlassianAuth,
	confluenceClient *atlassian.ConfluenceClient,
	jiraClient *atlassian.JiraClient,
	docProcessor *document.Processor,
	searchEngine *search.Engine,
	logger *log.Logger,
	sessionManager *session.SessionManager,
) *Handler {
	return &Handler{
		atlassianAuth:    atlassianAuth,
		confluenceClient: confluenceClient,
		jiraClient:       jiraClient,
		docProcessor:     docProcessor,
		searchEngine:     searchEngine,
		logger:           logger,
		sessionManager:   sessionManager,
	}
}

// HealthCheck provides a simple health check endpoint
func (h *Handler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// AtlassianLoginURL generates the login URL for Atlassian OAuth
func (h *Handler) AtlassianLoginURL(c *gin.Context) {
	// Log incoming headers and cookies for debugging
	h.logger.Printf("Login request headers: %v", c.Request.Header)
	cookies := c.Request.Cookies()
	h.logger.Printf("Login request contains %d cookies", len(cookies))
	for i, cookie := range cookies {
		h.logger.Printf("Cookie %d: Name=%s, Value=%s", i, cookie.Name, cookie.Value)
	}

	// Generate authorization URL
	host := c.Request.Host
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	redirectURI := fmt.Sprintf("%s://%s/auth/callback", scheme, host)
	h.logger.Printf("Using redirect URI: %s", redirectURI)

	// Generate state parameter using session manager
	state, err := h.sessionManager.GenerateState(c)
	if err != nil {
		h.logger.Printf("Failed to generate state: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate state"})
		return
	}

	// After generating state, check cookies again to verify it was set
	h.logger.Printf("Response before writing contains the following cookies:")
	for _, cookie := range c.Writer.Header()["Set-Cookie"] {
		h.logger.Printf("Set-Cookie: %s", cookie)
	}

	// Generate authorization URL
	authURL := h.atlassianAuth.GetAuthURL(redirectURI, state)
	h.logger.Printf("Generated auth URL with state: %s", state)

	// Return authorization URL
	c.JSON(http.StatusOK, gin.H{"url": authURL})
}

// AtlassianCallback handles the callback from Atlassian OAuth
func (h *Handler) AtlassianCallback(c *gin.Context) {
	// Log incoming headers and cookies for debugging
	h.logger.Printf("Callback request headers: %v", c.Request.Header)
	cookies := c.Request.Cookies()
	h.logger.Printf("Callback request contains %d cookies", len(cookies))
	for i, cookie := range cookies {
		h.logger.Printf("Cookie %d: Name=%s, Value=%s", i, cookie.Name, cookie.Value)
	}

	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Authorization code is required"})
		return
	}

	state := c.Query("state")
	if state == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "State parameter is required"})
		return
	}

	// Generate the same redirect URI we used for the initial request
	host := c.Request.Host
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	redirectURI := fmt.Sprintf("%s://%s/auth/callback", scheme, host)
	h.logger.Printf("Using redirect URI: %s", redirectURI)

	h.logger.Printf("Received callback: code=%s, state=%s", code, state)

	// Validate state using session manager
	if err := h.sessionManager.ValidateState(c, state); err != nil {
		h.logger.Printf("State validation failed: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Exchange code for token
	tokenResponse, err := h.atlassianAuth.ExchangeCodeForToken(c.Request.Context(), code, redirectURI)
	if err != nil {
		h.logger.Printf("Token exchange failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to exchange token"})
		return
	}

	// Store the access token in the session
	h.logger.Printf("Attempting to store token in session")
	if err := h.sessionManager.StoreToken(c, tokenResponse.AccessToken); err != nil {
		h.logger.Printf("Failed to store token: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store token"})
		return
	}
	h.logger.Printf("Token stored successfully in session")

	// Get user info
	userInfo, err := h.atlassianAuth.GetUserInfo(c.Request.Context(), tokenResponse.AccessToken)
	if err != nil {
		h.logger.Printf("Failed to get user info: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user info"})
		return
	}

	// Return success response with token info
	c.JSON(http.StatusOK, gin.H{
		"message": "Authentication successful",
		"token":   tokenResponse,
		"user":    userInfo,
	})
}

// UploadDocument handles document upload and processing
func (h *Handler) UploadDocument(c *gin.Context) {
	// Get user from context
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	atlassianUser, ok := user.(*auth.UserInfo)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user type"})
		return
	}

	// Get token from context
	token, exists := c.Get("token")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Token not found"})
		return
	}

	_, ok = token.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid token type"})
		return
	}

	// Get file from request
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File is required"})
		return
	}
	defer file.Close()

	// Process document
	result, err := h.docProcessor.ProcessFile(c.Request.Context(), file, header)
	if err != nil {
		h.logger.Printf("Document processing failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process document"})
		return
	}

	// Create user permissions for this document
	// In a real implementation, you would use actual permissions
	// For POC, we'll use a simple approach
	permissions := []string{atlassianUser.AccountID, result.DocumentID}

	// Index document for search
	err = h.searchEngine.IndexDocument(c.Request.Context(), result, permissions)
	if err != nil {
		h.logger.Printf("Document indexing failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to index document"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"document_id": result.DocumentID,
		"title":       result.Title,
		"chunks":      len(result.Content),
		"metadata":    result.Metadata,
	})
}

// Search handles semantic search requests
func (h *Handler) Search(c *gin.Context) {
	// Get user from context
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	atlassianUser, ok := user.(*auth.UserInfo)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user type"})
		return
	}

	// Parse search request
	var req struct {
		Query string `json:"query" binding:"required"`
		Limit int    `json:"limit"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Get user permissions
	// In a real implementation, you would fetch actual permissions from Atlassian
	// For POC, we'll use a simple approach
	permissions := []string{atlassianUser.AccountID}

	// Perform search
	results, err := h.searchEngine.Search(c.Request.Context(), &search.SearchRequest{
		Query:       req.Query,
		UserID:      atlassianUser.AccountID,
		Permissions: permissions,
		Limit:       req.Limit,
	})

	if err != nil {
		h.logger.Printf("Search failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Search failed"})
		return
	}

	// Format results
	formattedResults := make([]gin.H, len(results))
	for i, result := range results {
		formattedResults[i] = gin.H{
			"document_id": result.DocumentID,
			"title":       result.Title,
			"content":     result.ChunkContent,
			"score":       result.Score,
			"metadata":    result.Metadata,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"results": formattedResults,
		"count":   len(results),
	})
}

// ListConfluenceSpaces lists Confluence spaces
func (h *Handler) ListConfluenceSpaces(c *gin.Context) {
	// Get token from context
	token, exists := c.Get("token")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Token not found"})
		return
	}

	_, ok := token.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid token type"})
		return
	}

	// List spaces
	spaces, err := h.confluenceClient.ListSpaces(c.Request.Context(), token.(string))
	if err != nil {
		h.logger.Printf("List spaces failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list spaces"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"spaces": spaces,
		"count":  len(spaces),
	})
}

// ListConfluencePages lists Confluence pages in a space
func (h *Handler) ListConfluencePages(c *gin.Context) {
	// Get token from context
	token, exists := c.Get("token")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Token not found"})
		return
	}

	_, ok := token.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid token type"})
		return
	}

	// Get space key from path
	spaceKey := c.Param("spaceKey")
	if spaceKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Space key is required"})
		return
	}

	// List pages
	pages, err := h.confluenceClient.ListPages(c.Request.Context(), token.(string), spaceKey)
	if err != nil {
		h.logger.Printf("List pages failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list pages"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"pages": pages,
		"count": len(pages),
	})
}

// ProcessConfluencePage processes a Confluence page for search
func (h *Handler) ProcessConfluencePage(c *gin.Context) {
	// Get user from context
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	atlassianUser, ok := user.(*auth.UserInfo)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user type"})
		return
	}

	// Get token from context
	token, exists := c.Get("token")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Token not found"})
		return
	}

	_, ok = token.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid token type"})
		return
	}

	// Get page ID from path
	pageID := c.Param("pageId")
	if pageID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Page ID is required"})
		return
	}

	// Get page content
	page, err := h.confluenceClient.GetPageContent(c.Request.Context(), token.(string), pageID)
	if err != nil {
		h.logger.Printf("Get page content failed for pageID=%s: %v", pageID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get page content", "details": err.Error()})
		return
	}

	// Process page content
	result, err := h.docProcessor.ProcessConfluencePage(c.Request.Context(), pageID, page.Title, page.Body.Storage.Value)
	if err != nil {
		h.logger.Printf("Page processing failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process page"})
		return
	}

	// Get page permissions
	permissions, err := h.confluenceClient.GetPagePermissions(c.Request.Context(), token.(string), pageID)
	if err != nil {
		h.logger.Printf("Get page permissions failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get page permissions"})
		return
	}

	// Add user ID to permissions
	permissions = append(permissions, atlassianUser.AccountID)

	// Index document for search
	err = h.searchEngine.IndexDocument(c.Request.Context(), result, permissions)
	if err != nil {
		h.logger.Printf("Page indexing failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to index page"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"document_id": result.DocumentID,
		"title":       result.Title,
		"chunks":      len(result.Content),
		"metadata":    result.Metadata,
	})
}

// ListJiraProjects lists Jira projects
func (h *Handler) ListJiraProjects(c *gin.Context) {
	// Get token from context
	token, exists := c.Get("token")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Token not found"})
		return
	}

	_, ok := token.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid token type"})
		return
	}

	// List projects
	projects, err := h.jiraClient.ListProjects(c.Request.Context(), token.(string))
	if err != nil {
		h.logger.Printf("List projects failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list projects"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"projects": projects,
		"count":    len(projects),
	})
}

// CreateJiraTicket creates a Jira ticket
func (h *Handler) CreateJiraTicket(c *gin.Context) {
	// Get token from context
	token, exists := c.Get("token")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Token not found"})
		return
	}

	_, ok := token.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid token type"})
		return
	}

	// Parse request
	var req struct {
		ProjectKey  string `json:"project_key" binding:"required"`
		Summary     string `json:"summary" binding:"required"`
		Description string `json:"description" binding:"required"`
		IssueTypeID string `json:"issue_type_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Create ticket
	ticket, err := h.jiraClient.CreateIssue(
		c.Request.Context(),
		token.(string),
		req.ProjectKey,
		req.Summary,
		req.Description,
		req.IssueTypeID,
	)

	if err != nil {
		h.logger.Printf("Create ticket failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create ticket"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ticket_id":  ticket.ID,
		"ticket_key": ticket.Key,
		"self":       ticket.Self,
	})
}

// TestTokenExchange tests the token exchange process
func (h *Handler) TestTokenExchange(c *gin.Context) {
	// Get the authorization code from the query parameters
	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "authorization code is required"})
		return
	}

	// Use a fixed redirect URI for testing
	redirectURI := "http://localhost:8080/auth/callback"

	// Exchange the code for an access token
	token, err := h.atlassianAuth.ExchangeCodeForToken(c.Request.Context(), code, redirectURI)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to exchange code for token: %v", err)})
		return
	}

	// Return the token details (excluding sensitive information)
	c.JSON(http.StatusOK, gin.H{
		"token_type": token.TokenType,
		"expires_in": token.ExpiresIn,
		"scope":      token.Scope,
	})
}

// SessionManager returns the session manager
func (h *Handler) SessionManager() *session.SessionManager {
	return h.sessionManager
}
