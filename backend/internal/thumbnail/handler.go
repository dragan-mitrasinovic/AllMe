package thumbnail

import (
	"all-me-backend/internal/apierror"
	"all-me-backend/internal/session"
	"all-me-backend/internal/urlguard"
	"all-me-backend/pkg/models"
	"fmt"
	"io"
	"net/http"

	"github.com/labstack/echo/v4"
)

type Handler struct {
	sessionStore       models.SessionStore
	googleDriveService Provider
	oneDriveService    Provider
}

func NewHandler(sessionStore models.SessionStore, googleDriveService Provider, oneDriveService Provider) *Handler {
	return &Handler{
		sessionStore:       sessionStore,
		googleDriveService: googleDriveService,
		oneDriveService:    oneDriveService,
	}
}

func (h *Handler) RegisterRoutes(e *echo.Echo) {
	e.GET("/thumbnail", h.handleThumbnailProxy, session.Required)
}

func (h *Handler) handleThumbnailProxy(c echo.Context) error {
	sessionID := session.FromContext(c)
	thumbnailURL := c.QueryParam("url")
	provider := c.QueryParam("provider")

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

	allowlist, err := urlguard.ForProvider(provider)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("unsupported provider: %s", provider),
		})
	}
	if _, err := allowlist.Check(thumbnailURL); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "url is not a thumbnail URL of the selected provider",
		})
	}

	// Get token from session
	token, err := h.sessionStore.GetSessionToken(sessionID, provider)
	if err != nil {
		return apierror.NotSignedIn(c, err)
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
		return apierror.Respond(c, http.StatusBadGateway, "Couldn't load the thumbnail.", err)
	}
	defer thumbnailStream.Close()

	// Set cache headers
	c.Response().Header().Set("Cache-Control", "public, max-age=3600") // Cache for 1 hour
	c.Response().Header().Set("Content-Type", "image/jpeg")            // Default to JPEG, could be improved

	_, err = io.Copy(c.Response().Writer, thumbnailStream)
	return err
}
