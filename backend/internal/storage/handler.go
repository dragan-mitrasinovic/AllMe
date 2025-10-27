package storage

import (
	"all-me-backend/pkg/models"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
)

// Handler handles HTTP requests for storage operations
type Handler struct {
	service      *Service
	sessionStore models.SessionStore
}

// NewHandler creates a new storage handler
func NewHandler(service *Service, sessionStore models.SessionStore) *Handler {
	return &Handler{
		service:      service,
		sessionStore: sessionStore,
	}
}

// RegisterRoutes registers storage routes with the Echo router
func (h *Handler) RegisterRoutes(e *echo.Echo) {
	e.GET("/storage/folder-contents", h.GetFolderContents)
}

// GetFolderContents handles GET /storage/folder-contents
// It retrieves folder metadata and all contents (files and folders) from a cloud storage share link
func (h *Handler) GetFolderContents(c echo.Context) error {
	shareURL := c.QueryParam("share_url")
	sessionID := c.QueryParam("session_id")
	provider := c.QueryParam("provider")

	if shareURL == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "share_url query parameter is required",
		})
	}

	if sessionID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "session_id query parameter is required",
		})
	}

	if provider == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "provider query parameter is required",
		})
	}

	// Get token from session
	token, err := h.sessionStore.GetSessionToken(sessionID, provider)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": fmt.Sprintf("Authentication failed: %v", err),
		})
	}

	// Parse the share link to get folder information
	folder, err := h.service.ParseShareLink(shareURL, token)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("Failed to parse share link: %v", err),
		})
	}

	// List all contents in the folder
	contents, err := h.service.ListFolderContents(folder, token)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("Failed to list folder contents: %v", err),
		})
	}

	// Return folder metadata and all contents
	return c.JSON(http.StatusOK, GetFolderContentsResponse{
		Folder:   folder,
		Contents: contents,
	})
}
