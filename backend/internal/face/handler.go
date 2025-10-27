package face

import (
	"all-me-backend/pkg/models"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

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
	face := e.Group("/face")

	face.POST("/register-base", h.RegisterBaseFace)
	face.POST("/compare-folder", h.CompareFolder)
	face.GET("/job-status/:jobId", h.GetJobStatus)
	face.DELETE("/clear-reference/:sessionId", h.ClearReferenceImage)
}

func (h *Handler) RegisterBaseFace(c echo.Context) error {
	var req RegisterBaseFaceRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "Invalid request format",
		})
	}

	if err := validateRegisterRequest(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
	}

	file, err := c.FormFile("image")
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "Image file is required",
		})
	}

	if err := validateImageFile(file); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
	}

	src, err := file.Open()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error": "Failed to process image file",
		})
	}
	defer src.Close()

	imageData, err := io.ReadAll(src)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"error": "Failed to read image file",
		})
	}

	if err := h.service.RegisterBaseFace(req.SessionID, imageData); err != nil {
		return handleServiceError(c, err)
	}

	return c.JSON(http.StatusOK, RegisterBaseFaceResponse{
		Success: true,
	})
}

func (h *Handler) CompareFolder(c echo.Context) error {
	var req CompareFolderRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "Invalid request format",
		})
	}

	if err := validateCompareFolderRequest(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": err.Error(),
		})
	}

	token, err := h.sessionStore.GetSessionToken(req.SessionID, req.Provider)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, echo.Map{
			"error": fmt.Sprintf("Authentication failed: %v", err),
		})
	}

	jobID, err := h.service.CompareFolderImages(req.SessionID, req.FolderLink, token, req.Recursive)
	if err != nil {
		return handleServiceError(c, err)
	}

	return c.JSON(http.StatusOK, CompareFolderResponse{
		JobID:  jobID,
		Status: "processing",
	})
}

func (h *Handler) GetJobStatus(c echo.Context) error {
	jobID := c.Param("jobId")

	if strings.TrimSpace(jobID) == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "job_id is required",
		})
	}

	status, err := h.service.GetJobStatus(jobID)
	if err != nil {
		return handleServiceError(c, err)
	}

	return c.JSON(http.StatusOK, status)
}

func (h *Handler) ClearReferenceImage(c echo.Context) error {
	sessionID := c.Param("sessionId")

	if strings.TrimSpace(sessionID) == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "session_id is required",
		})
	}

	if err := h.service.ClearReferenceImage(sessionID); err != nil {
		return handleServiceError(c, err)
	}

	return c.JSON(http.StatusOK, echo.Map{
		"success": true,
		"message": "Reference image cleared successfully",
	})
}

func validateRegisterRequest(req *RegisterBaseFaceRequest) error {
	if strings.TrimSpace(req.SessionID) == "" {
		return errors.New("session_id is required")
	}
	return nil
}

func validateCompareFolderRequest(req *CompareFolderRequest) error {
	if strings.TrimSpace(req.SessionID) == "" {
		return errors.New("session_id is required")
	}

	if strings.TrimSpace(req.FolderLink) == "" {
		return errors.New("folder_link is required")
	}

	if strings.TrimSpace(req.Provider) == "" {
		return errors.New("provider is required")
	}

	return nil
}

func validateImageFile(file *multipart.FileHeader) error {
	const maxFileSize = 20 * 1024 * 1024 // 20MB

	if file.Size > maxFileSize {
		return errors.New("image file size exceeds maximum allowed size of 20MB")
	}

	if file.Size == 0 {
		return errors.New("image file is empty")
	}

	contentType := file.Header.Get("Content-Type")
	validTypes := map[string]bool{
		"image/jpeg": true,
		"image/jpg":  true,
		"image/png":  true,
		"image/heic": true,
		"image/heif": true,
	}

	if !validTypes[contentType] {
		return errors.New("invalid image format. Supported formats: JPEG, PNG, HEIC")
	}

	return nil
}

func handleServiceError(c echo.Context, err error) error {
	errResp := GetErrorResponse(err)
	return c.JSON(errResp.StatusCode, echo.Map{
		"error": errResp.Message,
	})
}
