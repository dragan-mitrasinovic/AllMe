package download

import (
	"all-me-backend/pkg/models"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

// Handler handles HTTP requests for download operations
type Handler struct {
	service      *Service
	sessionStore models.SessionStore
}

// NewHandler creates a new download handler
func NewHandler(service *Service, sessionStore models.SessionStore) *Handler {
	return &Handler{
		service:      service,
		sessionStore: sessionStore,
	}
}

// RegisterRoutes registers download routes with the Echo router
func (h *Handler) RegisterRoutes(e *echo.Echo) {
	e.POST("/downloads/zip", h.DownloadZip)
}

// DownloadZip handles POST /downloads/zip
// It streams multiple files as a ZIP archive directly to the response
func (h *Handler) DownloadZip(c echo.Context) error {
	var req ZipRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid request body",
		})
	}

	if len(req.Files) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "No files provided for download",
		})
	}

	if req.SessionID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Session ID is required",
		})
	}

	if req.Provider == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Provider is required",
		})
	}

	// Get token from session
	session, err := h.sessionStore.GetSession(req.SessionID)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "Session not found or expired",
		})
	}

	// Get the token for the requested provider
	token := session.GetToken(req.Provider)
	if token == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": fmt.Sprintf("No authentication token found for provider %s", req.Provider),
		})
	}

	// Set appropriate headers for ZIP download
	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("photos-%s.zip", timestamp)

	c.Response().Header().Set("Content-Type", "application/zip")
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Response().WriteHeader(http.StatusOK)

	// Stream the ZIP archive directly to the response
	if err := h.service.StreamZipArchive(c.Response().Writer, req.Files, token); err != nil {
		// At this point headers are already sent, so we can't return a proper error response
		// Log the error and the connection will be closed
		c.Logger().Errorf("Failed to stream ZIP archive: %v", err)
		return nil
	}

	return nil
}
