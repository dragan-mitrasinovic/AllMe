package auth

import (
	"all-me-backend/internal/apierror"
	"all-me-backend/internal/session"
	"net/http"
	"net/url"
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

	if !h.authService.validateProvider(provider) {
		return apierror.Respond(c, http.StatusBadRequest, "Unsupported cloud storage provider.", nil)
	}

	authURL, err := h.authService.InitiateOAuth(provider, sessionID)
	if err != nil {
		return apierror.Respond(c, http.StatusInternalServerError,
			"Signing in isn't available right now. Please try again later.", err)
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

	// The provider reported an error (e.g. the user cancelled): pass on only a known code
	if errorParam != "" {
		c.Logger().Warnf("OAuth error from %s: %s: %s", provider, errorParam, c.QueryParam("error_description"))
		if errorParam != "access_denied" {
			errorParam = "auth_failed"
		}
		return h.redirectToCallback(c, url.Values{"error": {errorParam}})
	}

	if code == "" || state == "" {
		return h.redirectToCallback(c, url.Values{"error": {"auth_failed"}})
	}

	token, newSessionID, err := h.authService.HandleCallback(provider, code, state, session.FromContext(c))
	if err != nil {
		c.Logger().Errorf("OAuth callback for %s failed: %v", provider, err)
		return h.redirectToCallback(c, url.Values{"error": {"auth_failed"}})
	}

	session.SetCookie(c, newSessionID)

	return h.redirectToCallback(c, url.Values{"success": {"true"}, "provider": {token.Provider}})
}

// redirectToCallback sends the browser to the frontend's /callback page with properly encoded parameters.
func (h *Handler) redirectToCallback(c echo.Context, params url.Values) error {
	return c.Redirect(http.StatusTemporaryRedirect, h.frontendURL+"/callback?"+params.Encode())
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
			return apierror.Respond(c, http.StatusBadRequest, "Unsupported cloud storage provider.", err)
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
