package storage

import (
	"all-me-backend/internal/apierror"
	"all-me-backend/internal/session"
	"all-me-backend/pkg/models"
	"net/http"

	"github.com/labstack/echo/v4"
)

type Handler struct {
	service      *Service
	sessionStore models.SessionStore
}

func NewHandler(service *Service, sessionStore models.SessionStore) *Handler {
	return &Handler{
		service:      service,
		sessionStore: sessionStore,
	}
}

func (h *Handler) RegisterRoutes(e *echo.Echo) {
	e.GET("/storage/folder-contents", h.GetFolderContents, session.Required)
}

// GetFolderContents handles GET /storage/folder-contents
// It retrieves folder metadata and all contents (files and folders) from a cloud storage share link
func (h *Handler) GetFolderContents(c echo.Context) error {
	shareURL := c.QueryParam("share_url")
	sessionID := session.FromContext(c)
	provider := c.QueryParam("provider")

	if shareURL == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "share_url query parameter is required",
		})
	}

	if provider == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "provider query parameter is required",
		})
	}

	token, err := h.sessionStore.GetSessionToken(sessionID, provider)
	if err != nil {
		return apierror.NotSignedIn(c, err)
	}

	folder, err := h.service.ParseShareLink(shareURL, token)
	if err != nil {
		return apierror.Respond(c, http.StatusBadRequest,
			"Couldn't open that folder link. Check that it's a shared folder link and that you can open it.", err)
	}

	contents, err := h.service.ListFolderContents(folder, token)
	if err != nil {
		return apierror.Respond(c, http.StatusBadGateway,
			"Couldn't read the folder from your cloud storage. Please try again.", err)
	}

	return c.JSON(http.StatusOK, GetFolderContentsResponse{
		Folder:   folder,
		Contents: contents,
	})
}
