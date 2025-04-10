package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"
	"github.com/sanjeevkumarraob/semantic-search-service/internal/api"
	"github.com/sanjeevkumarraob/semantic-search-service/internal/atlassian"
	"github.com/sanjeevkumarraob/semantic-search-service/internal/auth"
	"github.com/sanjeevkumarraob/semantic-search-service/internal/document"
	"github.com/sanjeevkumarraob/semantic-search-service/internal/search"
	"github.com/sanjeevkumarraob/semantic-search-service/internal/session"
)

func main() {
	// Initialize logger
	logger := log.New(os.Stdout, "SEMANTIC-SEARCH: ", log.Ldate|log.Ltime|log.Lshortfile)

	// Initialize Atlassian auth
	clientID := os.Getenv("ATLASSIAN_CLIENT_ID")
	clientSecret := os.Getenv("ATLASSIAN_CLIENT_SECRET")
	atlassianAuth := auth.NewAtlassianAuth(clientID, clientSecret)

	// Initialize Atlassian clients
	confluenceBaseURL := os.Getenv("CONFLUENCE_BASE_URL")
	if confluenceBaseURL == "" {
		// Default to your Atlassian site URL with /wiki included
		confluenceBaseURL = "https://sanjeevkumarrao.atlassian.net/wiki"
		logger.Printf("Using default Confluence base URL: %s", confluenceBaseURL)
	}
	jiraBaseURL := os.Getenv("JIRA_BASE_URL")
	if jiraBaseURL == "" {
		jiraBaseURL = "https://api.atlassian.com" // Default to Atlassian Cloud API
		logger.Printf("Using default Atlassian Cloud API base URL: %s", jiraBaseURL)
	}
	confluenceClient := atlassian.NewConfluenceClient(confluenceBaseURL)
	jiraClient := atlassian.NewJiraClient(jiraBaseURL)

	// Set cloud ID for Confluence client
	cloudID := os.Getenv("ATLASSIAN_CLOUD_ID")
	if cloudID == "" {
		logger.Printf("WARNING: ATLASSIAN_CLOUD_ID not set. Confluence API calls will fail.")
	} else {
		confluenceClient.SetCloudID(cloudID)
		logger.Printf("Confluence client configured with cloud ID: %s", cloudID)
	}

	// Initialize document processor
	docProcessor := document.NewProcessor(logger)

	// Initialize search engine
	searchEngine := search.NewEngine(logger)

	// Create a secure key for sessions
	key := []byte(os.Getenv("SESSION_SECRET"))
	if len(key) == 0 {
		key = []byte("your-secret-key") // Fallback for development
	}

	// Initialize session store
	store := sessions.NewCookieStore(key)
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   3600,
		HttpOnly: true,
		Secure:   true, // Must be true when using SameSiteNone
		SameSite: http.SameSiteNoneMode,
	}

	// Initialize session manager
	sessionManager := session.NewSessionManager(logger, store)

	// Initialize handler
	handler := api.NewHandler(
		atlassianAuth,
		confluenceClient,
		jiraClient,
		docProcessor,
		searchEngine,
		logger,
		sessionManager,
	)

	// Configure server
	router := gin.Default()

	// Configure routes
	router.GET("/", handler.HealthCheck)
	router.GET("/auth/login", handler.AtlassianLoginURL)
	router.GET("/auth/callback", handler.AtlassianCallback)

	// API routes that require authentication
	apiGroup := router.Group("/api")
	apiGroup.Use(api.AuthMiddleware(atlassianAuth, store))
	apiGroup.POST("/search", handler.Search)

	// Add Confluence endpoints
	apiGroup.GET("/confluence/spaces", handler.ListConfluenceSpaces)
	apiGroup.GET("/confluence/pages/:spaceKey", handler.ListConfluencePages)
	apiGroup.POST("/confluence/process/:pageId", handler.ProcessConfluencePage)

	// Add middleware to check for localhost in each request
	router.Use(func(c *gin.Context) {
		// Check if we're in a development environment
		isLocalhost := c.Request.Host == "localhost:8080" || c.Request.Host == "127.0.0.1:8080"

		// Update store options for this request
		store.Options = &sessions.Options{
			Path:     "/",
			MaxAge:   3600,
			HttpOnly: true,
			Secure:   !isLocalhost, // Only false for localhost
			SameSite: http.SameSiteNoneMode,
		}

		c.Next()
	})

	// Start server
	server := &http.Server{
		Addr:         ":8080",
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	logger.Printf("Server starting on %s", server.Addr)
	if err := server.ListenAndServe(); err != nil {
		logger.Fatalf("Server failed to start: %v", err)
	}
}
