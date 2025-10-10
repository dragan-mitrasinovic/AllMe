package auth

import (
	"net/http"
	"os"
	"time"

	"github.com/labstack/echo/v4"
)

// Handler handles authentication-related HTTP requests
type Handler struct {
	authService *Service
	frontendURL string
}

// NewHandler creates a new Handler instance
func NewHandler(authService *Service) *Handler {
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		// Fallback to localhost for local development
		frontendURL = "http://localhost:4200"
	}

	return &Handler{
		authService: authService,
		frontendURL: frontendURL,
	}
}

// RegisterRoutes registers authentication routes with the Echo instance
func (h *Handler) RegisterRoutes(e *echo.Echo) {
	e.GET("/auth/:provider/login", h.handleLogin)
	e.GET("/auth/:provider/callback", h.handleCallback)
	e.GET("/auth/validate-session", h.handleValidateSession)
	e.DELETE("/auth/signout", h.handleSignOut)
	e.GET("/health", h.handleHealth)
}

// handleLogin initiates the OAuth flow for a provider
func (h *Handler) handleLogin(c echo.Context) error {
	provider := c.Param("provider")
	sessionID := c.QueryParam("session_id")

	if sessionID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "session_id is required",
		})
	}

	authURL, err := h.authService.InitiateOAuth(provider, sessionID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}

	// Redirect to the provider's OAuth page
	return c.Redirect(http.StatusTemporaryRedirect, authURL)
}

// handleCallback processes the OAuth callback from OneDrive or Google Drive
func (h *Handler) handleCallback(c echo.Context) error {
	provider := c.Param("provider")
	code := c.QueryParam("code")
	state := c.QueryParam("state")
	errorParam := c.QueryParam("error")

	// Handle OAuth errors - redirect to frontend with error
	if errorParam != "" {
		errorDescription := c.QueryParam("error_description")
		return c.Redirect(http.StatusTemporaryRedirect,
			h.frontendURL+"/callback?error="+errorParam+"&error_description="+errorDescription)
	}

	if code == "" {
		return c.Redirect(http.StatusTemporaryRedirect,
			h.frontendURL+"/callback?error=missing_code")
	}

	if state == "" {
		return c.Redirect(http.StatusTemporaryRedirect,
			h.frontendURL+"/callback?error=missing_state")
	}

	token, err := h.authService.HandleCallback(provider, code, state)
	if err != nil {
		return c.Redirect(http.StatusTemporaryRedirect,
			h.frontendURL+"/callback?error=auth_failed&message="+err.Error())
	}

	// Redirect to frontend callback with success
	return c.Redirect(http.StatusTemporaryRedirect,
		h.frontendURL+"/callback?success=true&provider="+token.Provider)
}

// handleValidateSession checks if a session exists and has a valid token for the provider
func (h *Handler) handleValidateSession(c echo.Context) error {
	sessionID := c.QueryParam("session_id")
	provider := c.QueryParam("provider")

	if sessionID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "session_id is required",
		})
	}

	if provider == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "provider is required",
		})
	}

	session, err := h.authService.GetSession(sessionID)
	if err != nil {
		// Session doesn't exist or is expired
		return c.JSON(http.StatusOK, map[string]interface{}{
			"valid":         false,
			"requires_auth": true,
		})
	}

	// Check if session has a token for the requested provider
	if !session.HasTokenForProvider(provider) {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"valid":         false,
			"requires_auth": true,
		})
	}

	// Session is valid and has the right token
	return c.JSON(http.StatusOK, map[string]interface{}{
		"valid":         true,
		"requires_auth": false,
		"provider":      provider,
	})
}

// handleSignOut removes the token for a specific provider from the session
func (h *Handler) handleSignOut(c echo.Context) error {
	sessionID := c.QueryParam("session_id")
	provider := c.QueryParam("provider")

	if sessionID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "session_id is required",
		})
	}

	if provider == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "provider is required",
		})
	}

	err := h.authService.SignOutProvider(sessionID, provider)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success":  true,
		"provider": provider,
		"message":  "Successfully signed out from " + provider,
	})
}

// handleHealth returns the health status of the backend service
func (h *Handler) handleHealth(c echo.Context) error {
	sessionCount := h.authService.GetSessionCount()

	response := map[string]interface{}{
		"status":    "healthy",
		"sessions":  sessionCount,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	return c.JSON(http.StatusOK, response)
}
