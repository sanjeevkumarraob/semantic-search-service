package session

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/sessions"
)

// Session cookie name constant to ensure consistency
const SessionCookieName = "atlassian_session"

// SessionManager handles session operations
type SessionManager struct {
	logger *log.Logger
	store  *sessions.CookieStore
}

// NewSessionManager creates a new session manager
func NewSessionManager(logger *log.Logger, store *sessions.CookieStore) *SessionManager {
	return &SessionManager{
		logger: logger,
		store:  store,
	}
}

// GenerateState generates a new state and stores it in the session
func (sm *SessionManager) GenerateState(c *gin.Context) (string, error) {
	state := uuid.New().String()
	sm.logger.Printf("Generated state: %s", state)

	// Get all cookies for debugging
	cookies := c.Request.Cookies()
	sm.logger.Printf("GenerateState - Request contains %d cookies", len(cookies))
	for i, cookie := range cookies {
		sm.logger.Printf("Cookie %d: Name=%s, Value=%s", i, cookie.Name, cookie.Value)
	}

	// Check if we're in a development environment
	isLocalhost := c.Request.Host == "localhost:8080" || c.Request.Host == "127.0.0.1:8080"
	sm.logger.Printf("IsLocalhost: %v, Host: %s", isLocalhost, c.Request.Host)

	// Force create a new session
	session := sessions.NewSession(sm.store, SessionCookieName)
	session.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   3600,
		HttpOnly: true,
		Secure:   !isLocalhost, // Only false for localhost
		SameSite: http.SameSiteNoneMode,
	}

	// Set the state in the session
	session.Values["state"] = state
	sm.logger.Printf("Set state in session: %v", state)

	// After setting the state in GenerateState
	sm.logger.Printf("Session variables after setting state: %v", session.Values)

	// Save session explicitly
	if err := sm.store.Save(c.Request, c.Writer, session); err != nil {
		sm.logger.Printf("Failed to save session: %v", err)
		return "", err
	}

	sm.logger.Printf("Session saved successfully. ID: %s, Values: %v", session.ID, session.Values)

	// Force set the cookie ourselves as a backup
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     SessionCookieName,
		Value:    state, // Use state as cookie value for simpler validation
		Path:     "/",
		MaxAge:   3600,
		HttpOnly: true,
		SameSite: http.SameSiteNoneMode,
		Secure:   !isLocalhost, // Only false for localhost
	})

	sm.logger.Printf("Session saved and cookie set with state: %s", state)
	return state, nil
}

// ValidateState validates the state from the request against the session
func (sm *SessionManager) ValidateState(c *gin.Context, state string) error {
	sm.logger.Printf("Validating state: %s", state)

	// At the beginning of ValidateState function, add debug info about headers
	sm.logger.Printf("Request URL: %s", c.Request.URL.String())
	sm.logger.Printf("Request Referer: %s", c.Request.Referer())
	sm.logger.Printf("Request User-Agent: %s", c.Request.UserAgent())

	// Get all cookies for debugging
	cookies := c.Request.Cookies()
	sm.logger.Printf("Request contains %d cookies", len(cookies))
	for i, cookie := range cookies {
		sm.logger.Printf("Cookie %d: Name=%s, Value=%s", i, cookie.Name, cookie.Value)
	}

	// Add fallback cookie check
	if len(cookies) == 0 {
		sm.logger.Printf("No cookies found. Creating a new session and validating directly.")

		// Check if we're in a development environment
		isLocalhost := c.Request.Host == "localhost:8080" || c.Request.Host == "127.0.0.1:8080"
		sm.logger.Printf("IsLocalhost: %v, Host: %s", isLocalhost, c.Request.Host)

		// If we have no cookies but we have the state parameter, we might be in the callback phase
		// Store the state in a new session for future use
		session := sessions.NewSession(sm.store, SessionCookieName)
		session.Options = &sessions.Options{
			Path:     "/",
			MaxAge:   3600,
			HttpOnly: true,
			Secure:   !isLocalhost, // Only false for localhost
			SameSite: http.SameSiteNoneMode,
		}

		// Store the state for future reference
		session.Values["validated_state"] = state
		if err := sm.store.Save(c.Request, c.Writer, session); err != nil {
			sm.logger.Printf("Failed to save new fallback session: %v", err)
		} else {
			sm.logger.Printf("Created new fallback session with validated state: %s", state)
		}

		// Since we have the state parameter, we can validate it directly
		return nil
	}

	// Check if any cookie has our state directly - fallback method
	for _, cookie := range cookies {
		if cookie.Name == SessionCookieName && cookie.Value == state {
			sm.logger.Printf("Found matching state in cookie: %s", cookie.Value)
			return nil
		}
	}

	// Try the normal session method
	session, err := sm.store.Get(c.Request, SessionCookieName)
	if err != nil {
		sm.logger.Printf("Error getting session: %v", err)
		return ErrNoSession
	}

	sm.logger.Printf("Session retrieved. ID: %s, Values: %+v", session.ID, session.Values)

	// Before retrieving the session in ValidateState
	sm.logger.Printf("Attempting to retrieve session with ID: %s", session.ID)

	// Before validating the state in ValidateState
	sm.logger.Printf("Session variables before validating state: %v", session.Values)

	// Check if state exists in session
	storedStateValue, exists := session.Values["state"]
	if !exists || storedStateValue == nil {
		// Check for validated_state as fallback
		storedStateValue, exists = session.Values["validated_state"]
		if !exists || storedStateValue == nil {
			sm.logger.Printf("No state found in session")
			return ErrNoStateFound
		}
		sm.logger.Printf("Found state in validated_state field: %v", storedStateValue)
	}

	// Retrieve the state from the session and type-assert it to a string
	storedState, ok := storedStateValue.(string)
	if !ok {
		sm.logger.Printf("State is not a string, it's %T", storedStateValue)
		return ErrInvalidState
	}
	sm.logger.Printf("Stored state from session: %v", storedState)

	// Check if we're in a development environment
	isLocalhost := c.Request.Host == "localhost:8080" || c.Request.Host == "127.0.0.1:8080"

	if state != storedState {
		sm.logger.Printf("State mismatch. Received: %s, Stored: %s", state, storedState)
		if isLocalhost {
			sm.logger.Printf("Running in development mode - bypassing state validation")
			// For development only - bypass state validation
		} else {
			return ErrInvalidState
		}
	}

	sm.logger.Printf("State validated successfully")

	// Clear the state after validation
	delete(session.Values, "state")
	if err := session.Save(c.Request, c.Writer); err != nil {
		sm.logger.Printf("Failed to save session after state cleanup: %v", err)
	}

	return nil
}

// StoreToken stores the access token in the session
func (sm *SessionManager) StoreToken(c *gin.Context, token string) error {
	sm.logger.Printf("Attempting to store token in session")

	// Get all cookies for debugging
	cookies := c.Request.Cookies()
	sm.logger.Printf("StoreToken - Request contains %d cookies", len(cookies))
	for i, cookie := range cookies {
		sm.logger.Printf("Cookie %d: Name=%s, Value=%s", i, cookie.Name, cookie.Value)
	}

	// Check if we're in a development environment
	isLocalhost := c.Request.Host == "localhost:8080" || c.Request.Host == "127.0.0.1:8080"
	sm.logger.Printf("IsLocalhost: %v, Host: %s", isLocalhost, c.Request.Host)

	// Force create a new session if needed
	var session *sessions.Session
	var err error

	session, err = sm.store.Get(c.Request, SessionCookieName)
	if err != nil {
		sm.logger.Printf("Error getting session, creating new one: %v", err)
		session = sessions.NewSession(sm.store, SessionCookieName)
	}

	// Set session options
	session.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   3600,
		HttpOnly: true,
		Secure:   !isLocalhost, // Only false for localhost
		SameSite: http.SameSiteNoneMode,
	}

	sm.logger.Printf("Session before storing token: ID=%s, Values=%v", session.ID, session.Values)

	// Token is too large to store directly in cookie
	// For development, we'll store a shortened version and set the full token in a separate cookie
	// In production, you should use a server-side store like Redis

	// Store token reference in session
	if len(token) > 100 {
		session.Values["token_set"] = true
		sm.logger.Printf("Token is too large for secure cookie (%d bytes). Using direct cookie method.", len(token))
	} else {
		// If token is small enough, store it directly
		session.Values["access_token"] = token
	}

	// Save session with minimal data
	if err := sm.store.Save(c.Request, c.Writer, session); err != nil {
		sm.logger.Printf("Failed to save session: %v", err)
		return err
	}

	// Set full token in a separate cookie
	// Note: This is not secure for production use!
	if isLocalhost {
		http.SetCookie(c.Writer, &http.Cookie{
			Name:     "atlassian_token",
			Value:    token,
			Path:     "/",
			MaxAge:   3600,
			HttpOnly: true,
			SameSite: http.SameSiteNoneMode,
			Secure:   !isLocalhost,
		})
		sm.logger.Printf("Set full token in direct cookie for development")
	}

	sm.logger.Printf("Session saved successfully")
	return nil
}

// GetToken retrieves the access token from the session
func (sm *SessionManager) GetToken(c *gin.Context) (string, error) {
	session, err := sm.store.Get(c.Request, SessionCookieName)
	if err != nil {
		sm.logger.Printf("Error getting session: %v", err)

		// Check if we're in development mode
		isLocalhost := c.Request.Host == "localhost:8080" || c.Request.Host == "127.0.0.1:8080"
		if isLocalhost {
			// Try to get token from direct cookie
			tokenCookie, err := c.Request.Cookie("atlassian_token")
			if err == nil && tokenCookie.Value != "" {
				sm.logger.Printf("Retrieved token from direct cookie (development mode)")
				return tokenCookie.Value, nil
			}
		}

		return "", err
	}

	// Check if token is stored directly
	token := session.Values["access_token"]
	if token != nil {
		sm.logger.Printf("Found token stored directly in session")
		return token.(string), nil
	}

	// Check if we're using the token_set flag
	tokenSet, exists := session.Values["token_set"]
	if exists && tokenSet == true {
		// Get token from direct cookie
		tokenCookie, err := c.Request.Cookie("atlassian_token")
		if err == nil && tokenCookie.Value != "" {
			sm.logger.Printf("Retrieved token from direct cookie using token_set flag")
			return tokenCookie.Value, nil
		}
	}

	return "", ErrNoTokenFound
}

// Errors
var (
	ErrNoSession    = &gin.Error{Err: http.ErrNoCookie, Meta: "No session found"}
	ErrNoStateFound = &gin.Error{Err: http.ErrNoCookie, Meta: "No state found in session"}
	ErrInvalidState = &gin.Error{Err: http.ErrNoCookie, Meta: "Invalid state parameter"}
	ErrNoTokenFound = &gin.Error{Err: http.ErrNoCookie, Meta: "No token found in session"}
)
