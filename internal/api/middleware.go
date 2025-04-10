package api

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"
	"github.com/sanjeevkumarraob/semantic-search-service/internal/auth"
	"github.com/sanjeevkumarraob/semantic-search-service/internal/session"
)

// LoggerMiddleware creates a custom logging middleware
func LoggerMiddleware(logger *log.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Start timer
		start := time.Now()
		path := c.Request.URL.Path

		// Process request
		c.Next()

		// Calculate response time
		latency := time.Since(start)

		// Log request details
		logger.Printf(
			"[%s] %s %s | %d | %s | %s",
			c.ClientIP(),
			c.Request.Method,
			path,
			c.Writer.Status(),
			latency,
			c.Errors.String(),
		)
	}
}

// SessionMiddleware creates a middleware that handles session management
func SessionMiddleware(store *sessions.CookieStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		sess, err := store.Get(c.Request, session.SessionCookieName)
		if err != nil {
			log.Printf("Error getting session: %v", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to get session"})
			return
		}

		// Save session to context for handlers to use
		c.Set("session", sess)
		c.Next()
	}
}

// AuthMiddleware creates a middleware that validates the access token
func AuthMiddleware(atlassianAuth *auth.AtlassianAuth, store *sessions.CookieStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if we're in a development environment
		isLocalhost := c.Request.Host == "localhost:8080" || c.Request.Host == "127.0.0.1:8080"
		log.Printf("AuthMiddleware - Request to %s, isLocalhost: %v", c.Request.Host, isLocalhost)

		// Update store options for this request
		store.Options = &sessions.Options{
			Path:     "/",
			MaxAge:   3600,
			HttpOnly: true,
			Secure:   !isLocalhost, // Only false for localhost
			SameSite: http.SameSiteNoneMode,
		}

		// Get all cookies for debugging
		cookies := c.Request.Cookies()
		log.Printf("AuthMiddleware - Request contains %d cookies", len(cookies))
		for i, cookie := range cookies {
			log.Printf("Cookie %d: Name=%s, Value=%s", i, cookie.Name, cookie.Value)
		}

		// Check for Authorization header first (Bearer token)
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" && len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			token := authHeader[7:] // Remove "Bearer " prefix
			log.Printf("Found token in Authorization header")

			// Validate token with Atlassian
			userInfo, err := atlassianAuth.GetUserInfo(context.Background(), token)
			if err != nil {
				log.Printf("Token validation failed: %v", err)
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid access token"})
				return
			}

			// Store user info in context for handlers to use
			c.Set("user", userInfo)
			c.Set("token", token)
			c.Next()
			return
		}

		// Fallback to session token if no Authorization header
		sess, err := store.Get(c.Request, session.SessionCookieName)
		if err != nil {
			log.Printf("Error getting session in middleware: %v", err)

			// For development, check direct cookie
			if isLocalhost {
				tokenCookie, err := c.Request.Cookie("atlassian_token")
				if err == nil && tokenCookie.Value != "" {
					log.Printf("Found token in direct cookie (development mode)")
					token := tokenCookie.Value

					// Validate token with Atlassian
					userInfo, err := atlassianAuth.GetUserInfo(context.Background(), token)
					if err != nil {
						log.Printf("Token validation failed: %v", err)
						c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid access token"})
						return
					}

					// Store user info in context for handlers to use
					c.Set("user", userInfo)
					c.Set("token", token)
					c.Next()
					return
				}
			}

			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "No session found"})
			return
		}

		log.Printf("Session retrieved in middleware. ID: %s, Values: %v", sess.ID, sess.Values)

		// Check for access token in session
		tokenValue, exists := sess.Values["access_token"]
		if !exists || tokenValue == nil {
			log.Printf("No access token found in session")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "No access token found"})
			return
		}

		token, ok := tokenValue.(string)
		if !ok {
			log.Printf("Token is not a string, it's %T", tokenValue)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid access token format"})
			return

		}

		// Validate token with Atlassian
		userInfo, err := atlassianAuth.GetUserInfo(context.Background(), token)
		if err != nil {
			log.Printf("Token validation failed: %v", err)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid access token"})
			return
		}

		// Store user info in context for handlers to use
		c.Set("user", userInfo)
		c.Set("token", token)
		c.Next()
	}
}

// RateLimitMiddleware implements basic rate limiting
func RateLimitMiddleware() gin.HandlerFunc {
	// In a real implementation, you would use something like Redis
	// For the POC, we'll use a simple in-memory counter
	return func(c *gin.Context) {
		// This would be implemented with a proper rate limiting solution
		// For POC, we'll just pass through
		c.Next()
	}
}
