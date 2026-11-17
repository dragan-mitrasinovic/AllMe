package download

import (
	"all-me-backend/internal/apierror"
	"all-me-backend/internal/session"
	"all-me-backend/internal/urlguard"
	"all-me-backend/pkg/models"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

// MaxFilesPerZip is a generous cap on one ZIP download; it only stops runaway requests.
const MaxFilesPerZip = 1000

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
	e.POST("/downloads/zip", h.DownloadZip, session.Required)
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

	if len(req.Files) > MaxFilesPerZip {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("You can download up to %d photos at once. Please select fewer.", MaxFilesPerZip),
		})
	}

	if req.Provider == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Provider is required",
		})
	}

	allowlist, err := urlguard.ForProvider(req.Provider)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Unsupported provider",
		})
	}
	// File URLs come from the browser: check them all before anything is fetched or streamed
	for _, file := range req.Files {
		if file == nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "Invalid file entry",
			})
		}
		if _, err := allowlist.Check(file.DownloadURL); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "File URL is not a download URL of the selected provider",
			})
		}
	}

	token, err := h.sessionStore.GetSessionToken(session.FromContext(c), req.Provider)
	if err != nil {
		return apierror.NotSignedIn(c, err)
	}

	// Set appropriate headers for ZIP download
	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("photos-%s.zip", timestamp)

	c.Response().Header().Set("Content-Type", "application/zip")
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Response().WriteHeader(http.StatusOK)

	// Stream the ZIP archive directly to the response
	if err := h.service.StreamZipArchive(c.Response().Writer, req.Files, token); err != nil {
		c.Logger().Errorf("Failed to stream ZIP archive: %v", err)
		return nil
	}

	return nil
}
