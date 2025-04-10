package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/google/uuid"
)

// AtlassianAuth handles Atlassian OAuth authentication
type AtlassianAuth struct {
	clientID     string
	clientSecret string
	httpClient   *http.Client
	states       map[string]bool // Store multiple valid states
	stateMutex   sync.Mutex      // Protect state map access
}

// NewAtlassianAuth creates a new AtlassianAuth instance
func NewAtlassianAuth(clientID, clientSecret string) *AtlassianAuth {
	if clientID == "" {
		panic("ATLASSIAN_CLIENT_ID environment variable is required")
	}
	if clientSecret == "" {
		panic("ATLASSIAN_CLIENT_SECRET environment variable is required")
	}
	return &AtlassianAuth{
		clientID:     clientID,
		clientSecret: clientSecret,
		httpClient:   &http.Client{},
		states:       make(map[string]bool),
		stateMutex:   sync.Mutex{},
	}
}

// generateAndStoreState generates a new state value and stores it
func (a *AtlassianAuth) generateAndStoreState() string {
	a.stateMutex.Lock()
	defer a.stateMutex.Unlock()

	state := uuid.New().String()
	a.states[state] = true
	return state
}

// VerifyAndRemoveState verifies if a state is valid and removes it if it is
func (a *AtlassianAuth) VerifyAndRemoveState(state string) bool {
	a.stateMutex.Lock()
	defer a.stateMutex.Unlock()

	if valid := a.states[state]; valid {
		delete(a.states, state) // Remove the state after use
		return true
	}
	return false
}

// GetAuthURL returns the URL for Atlassian OAuth login
func (a *AtlassianAuth) GetAuthURL(redirectURI, state string) string {
	if a.clientID == "" {
		panic("client ID is not set")
	}

	// Build the URL exactly as shown in Atlassian documentation
	baseURL := "https://auth.atlassian.com/authorize"
	scope := "read:me read:confluence-space.summary read:confluence-props read:confluence-content.all read:confluence-content.summary read:confluence-content.permission read:confluence-user read:confluence-groups readonly:content.attachment:confluence"

	// URL encode each parameter separately
	encodedParams := url.Values{}
	encodedParams.Add("audience", "api.atlassian.com")
	encodedParams.Add("client_id", a.clientID)
	encodedParams.Add("scope", scope)
	encodedParams.Add("redirect_uri", redirectURI)
	encodedParams.Add("state", state) // Use the state passed from the handler
	encodedParams.Add("response_type", "code")
	encodedParams.Add("prompt", "consent")

	// Get the encoded string and replace + with %20
	encodedString := encodedParams.Encode()
	encodedString = strings.ReplaceAll(encodedString, "+", "%20")

	return baseURL + "?" + encodedString
}

// ExchangeCodeForToken exchanges an authorization code for an access token
func (a *AtlassianAuth) ExchangeCodeForToken(ctx context.Context, code, redirectURI string) (*TokenResponse, error) {
	// Log the parameters we're using
	fmt.Printf("Exchanging code for token - code: %s, redirect_uri: %s\n", code, redirectURI)

	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("client_id", a.clientID)
	data.Set("client_secret", a.clientSecret)
	data.Set("code", code)
	data.Set("redirect_uri", redirectURI)

	// Create the request with the correct content type
	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		"https://auth.atlassian.com/oauth/token",
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %w", err)
	}

	// Set the correct content type header
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read token response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed with status %d: %s", resp.StatusCode, string(body))
	}

	var token TokenResponse
	if err := json.Unmarshal(body, &token); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w, body: %s", err, string(body))
	}

	// Log token details (excluding sensitive information)
	fmt.Printf("Token exchange successful. Token type: %s, Expires in: %d, Scope: %s\n",
		token.TokenType, token.ExpiresIn, token.Scope)

	return &token, nil
}

// GetUserInfo retrieves user information using the access token
func (a *AtlassianAuth) GetUserInfo(ctx context.Context, accessToken string) (*UserInfo, error) {
	// First, validate token by getting accessible resources
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.atlassian.com/oauth/token/accessible-resources", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create accessible resources request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	req.Header.Set("Accept", "application/json")

	fmt.Printf("Validating token with accessible resources endpoint...\n")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute accessible resources request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read accessible resources response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to validate token: status %d, body: %s", resp.StatusCode, string(body))
	}

	// Now get user info
	req, err = http.NewRequestWithContext(ctx, "GET", "https://api.atlassian.com/me", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create user info request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	req.Header.Set("Accept", "application/json")

	fmt.Printf("Getting user info...\n")

	resp, err = a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute user info request: %w", err)
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read user info response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get user info: status %d, body: %s", resp.StatusCode, string(body))
	}

	var userInfo UserInfo
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return nil, fmt.Errorf("failed to decode user info: %w, body: %s", err, string(body))
	}

	return &userInfo, nil
}

// TokenResponse represents the OAuth token response
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

// UserInfo represents the user information from Atlassian
type UserInfo struct {
	AccountID   string `json:"account_id"`
	Email       string `json:"email"`
	Name        string `json:"name"`
	PictureURL  string `json:"picture"`
	AccountType string `json:"account_type"`
}
