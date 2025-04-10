package api

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"

	"github.com/sanjeevkumarraob/semantic-search-service/internal/atlassian"
	"github.com/sanjeevkumarraob/semantic-search-service/internal/auth"
	"github.com/sanjeevkumarraob/semantic-search-service/internal/document"
	"github.com/sanjeevkumarraob/semantic-search-service/internal/search"
	"github.com/sanjeevkumarraob/semantic-search-service/internal/session"
)

// NewRouter sets up the API router
func NewRouter(
	atlassianAuth *auth.AtlassianAuth,
	confluenceClient *atlassian.ConfluenceClient,
	jiraClient *atlassian.JiraClient,
	docProcessor *document.Processor,
	searchEngine *search.Engine,
	logger *log.Logger,
) *gin.Engine {
	// Create gin router
	router := gin.New()

	// Set up middleware
	router.Use(gin.Recovery())
	router.Use(LoggerMiddleware(logger))

	// Set up CORS
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Initialize session store
	key := []byte(os.Getenv("SESSION_SECRET"))
	if len(key) == 0 {
		key = []byte("your-secret-key") // Fallback for development
		logger.Printf("WARNING: Using insecure default session key. Set SESSION_SECRET environment variable for production.")
	}
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

	// Create handler
	handler := NewHandler(
		atlassianAuth,
		confluenceClient,
		jiraClient,
		docProcessor,
		searchEngine,
		logger,
		sessionManager,
	)

	// Add session middleware to set session in context
	router.Use(func(c *gin.Context) {
		// Print request info for debugging
		logger.Printf("Request: %s %s, User-Agent: %s",
			c.Request.Method, c.Request.URL.Path, c.Request.UserAgent())

		// Get or create session
		sessionCookieName := "atlassian_session" // Must match what's in session.SessionCookieName
		session, err := store.Get(c.Request, sessionCookieName)
		if err != nil {
			logger.Printf("Error getting session: %v", err)
			// Create a new session
			session = sessions.NewSession(store, sessionCookieName)
			session.Options = &sessions.Options{
				Path:     "/",
				MaxAge:   3600,
				HttpOnly: true,
				Secure:   true, // Must be true when using SameSiteNone
				SameSite: http.SameSiteNoneMode,
			}
		}

		// Save the session in the request context for handlers to use
		c.Set("session", session)

		// Check if we're in a development environment
		isLocalhost := c.Request.Host == "localhost:8080" || c.Request.Host == "127.0.0.1:8080"
		logger.Printf("Request to %s, isLocalhost: %v", c.Request.Host, isLocalhost)

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

	// Public routes
	router.GET("/", handler.HealthCheck)
	router.GET("/auth/login", handler.AtlassianLoginURL)
	router.GET("/auth/callback", handler.AtlassianCallback)

	// Auth required routes
	authorized := router.Group("/api")
	authorized.Use(AuthMiddleware(atlassianAuth, store))
	{
		// Document endpoints
		authorized.POST("/documents/upload", handler.UploadDocument)

		// Search endpoints
		authorized.POST("/search", handler.Search)

		// Confluence endpoints
		authorized.GET("/confluence/spaces", handler.ListConfluenceSpaces)
		authorized.GET("/confluence/pages/:spaceKey", handler.ListConfluencePages)
		authorized.POST("/confluence/process/:pageId", handler.ProcessConfluencePage)

		// Jira endpoints
		authorized.GET("/jira/projects", handler.ListJiraProjects)
		authorized.POST("/jira/ticket", handler.CreateJiraTicket)
	}

	return router
}
