package thumbnail

import (
	"all-me-backend/internal/auth"
	"fmt"
	"io"
	"net/http"

	"github.com/labstack/echo/v4"
)

// Handler handles thumbnail proxy requests
type Handler struct {
	authService        *auth.Service
	googleDriveService Provider
	oneDriveService    Provider
}

// NewHandler creates a new Handler instance
func NewHandler(authService *auth.Service, googleDriveService Provider, oneDriveService Provider) *Handler {
	return &Handler{
		authService:        authService,
		googleDriveService: googleDriveService,
		oneDriveService:    oneDriveService,
	}
}

// RegisterRoutes registers thumbnail routes with the Echo instance
func (h *Handler) RegisterRoutes(e *echo.Echo) {
	e.GET("/thumbnail", h.handleThumbnailProxy)
}

// handleThumbnailProxy proxies thumbnail requests with proper authentication
func (h *Handler) handleThumbnailProxy(c echo.Context) error {
	sessionID := c.QueryParam("session_id")
	thumbnailURL := c.QueryParam("url")
	provider := c.QueryParam("provider")

	if sessionID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "session_id is required",
		})
	}

	if thumbnailURL == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "url is required",
		})
	}

	if provider == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "provider is required",
		})
	}

	// Get token from session
	token, err := h.authService.GetSessionToken(sessionID, provider)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": fmt.Sprintf("Authentication failed: %v", err),
		})
	}

	// Delegate to the appropriate provider service
	var thumbnailStream io.ReadCloser
	switch provider {
	case "googledrive":
		thumbnailStream, err = h.googleDriveService.GetThumbnailStream(thumbnailURL, token)
	case "onedrive":
		thumbnailStream, err = h.oneDriveService.GetThumbnailStream(thumbnailURL, token)
	default:
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("unsupported provider: %s", provider),
		})
	}

	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("failed to fetch thumbnail: %v", err),
		})
	}
	defer thumbnailStream.Close()

	// Set cache headers
	c.Response().Header().Set("Cache-Control", "public, max-age=3600") // Cache for 1 hour
	c.Response().Header().Set("Content-Type", "image/jpeg")            // Default to JPEG, could be improved

	// Copy the response body
	_, err = io.Copy(c.Response().Writer, thumbnailStream)
	return err
}
