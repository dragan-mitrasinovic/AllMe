package auth

import (
	"all-me-backend/internal/session"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
)

type Handler struct {
	authService *Service
	frontendURL string
}

func NewHandler(authService *Service) *Handler {
	frontendURL := os.Getenv("FRONTEND_URL")

	return &Handler{
		authService: authService,
		frontendURL: frontendURL,
	}
}

func (h *Handler) RegisterRoutes(e *echo.Echo) {
	auth := e.Group("/auth")

	auth.GET("/:provider/login", h.handleLogin)
	auth.GET("/:provider/callback", h.handleCallback)
	auth.GET("/validate-session", h.handleValidateSession)
	auth.POST("/signout", h.handleSignOut)
}

// SessionMiddleware puts the session from the cookie into the request context if it is still valid.
// It never creates sessions; only login does.
func SessionMiddleware(authService *Service) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if id := session.CookieValue(c); id != "" && authService.ResolveSession(id) {
				session.SetContext(c, id)
			}
			return next(c)
		}
	}
}

// handleLogin initiates the OAuth flow by redirecting to the provider's auth page.
// The browser's session is created here if it doesn't have one yet.
func (h *Handler) handleLogin(c echo.Context) error {
	provider := c.Param("provider")

	sessionID := session.FromContext(c)
	if sessionID == "" {
		newID, err := h.authService.CreateSession()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "failed to start session",
			})
		}
		sessionID = newID
		session.SetCookie(c, sessionID)
		session.SetContext(c, sessionID)
	}

	authURL, err := h.authService.InitiateOAuth(provider, sessionID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}

	return c.Redirect(http.StatusTemporaryRedirect, authURL)
}

// handleCallback processes the OAuth callback from the provider
// On success or failure, redirects to the frontend with appropriate query parameters
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

	token, newSessionID, err := h.authService.HandleCallback(provider, code, state, session.FromContext(c))
	if err != nil {
		return c.Redirect(http.StatusTemporaryRedirect,
			h.frontendURL+"/callback?error=auth_failed&message="+err.Error())
	}

	session.SetCookie(c, newSessionID)

	// Redirect to frontend callback with success
	return c.Redirect(http.StatusTemporaryRedirect,
		h.frontendURL+"/callback?success=true&provider="+token.Provider)
}

// handleValidateSession checks if the session is valid and has a token for the specified provider
func (h *Handler) handleValidateSession(c echo.Context) error {
	provider := c.QueryParam("provider")

	if provider == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "provider is required",
		})
	}

	sessionID := session.FromContext(c)
	token, err := h.authService.GetSessionToken(sessionID, provider)
	if sessionID == "" || err != nil || token == nil {
		// Session doesn't exist, is expired, or lacks token for provider
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

// handleSignOut signs out from the specified provider. When no provider is left, the session ends.
func (h *Handler) handleSignOut(c echo.Context) error {
	provider := c.QueryParam("provider")

	if provider == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "provider is required",
		})
	}

	sessionEnded := true
	if sessionID := session.FromContext(c); sessionID != "" {
		ended, err := h.authService.SignOutProvider(sessionID, provider)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": err.Error(),
			})
		}
		sessionEnded = ended
	}

	if sessionEnded {
		session.ClearCookie(c)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success":  true,
		"provider": provider,
		"message":  "Successfully signed out from " + provider,
	})
}
