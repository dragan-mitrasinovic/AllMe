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

// Handler handles face comparison HTTP requests
type Handler struct {
	service      *Service
	sessionStore models.SessionStore
}

// NewHandler creates a new Handler instance
func NewHandler(service *Service, sessionStore models.SessionStore) *Handler {
	return &Handler{
		service:      service,
		sessionStore: sessionStore,
	}
}

// RegisterRoutes registers face comparison routes with the Echo instance
func (h *Handler) RegisterRoutes(e *echo.Echo) {
	e.POST("/face/register-base", h.RegisterBaseFace)
	e.POST("/face/compare-folder", h.CompareFolder)
	e.GET("/face/job-status/:jobId", h.GetJobStatus)
	e.DELETE("/face/clear-reference/:sessionId", h.ClearReferenceImage)
}

// RegisterBaseFace handles POST /face/register-base
func (h *Handler) RegisterBaseFace(c echo.Context) error {
	var req RegisterBaseFaceRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid request format",
		})
	}

	if err := validateRegisterRequest(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}

	file, err := c.FormFile("image")
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Image file is required",
		})
	}

	if err := validateImageFile(file); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}

	src, err := file.Open()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to process image file",
		})
	}
	defer src.Close()

	imageData, err := io.ReadAll(src)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
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

// CompareFolder handles POST /face/compare-folder
func (h *Handler) CompareFolder(c echo.Context) error {
	var req CompareFolderRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid request format",
		})
	}

	if err := validateCompareFolderRequest(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}

	// Get token from session
	token, err := h.sessionStore.GetSessionToken(req.SessionID, req.Provider)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{
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

// GetJobStatus handles GET /face/job-status/:jobId
func (h *Handler) GetJobStatus(c echo.Context) error {
	jobID := c.Param("jobId")

	if strings.TrimSpace(jobID) == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "job_id is required",
		})
	}

	status, err := h.service.GetJobStatus(jobID)
	if err != nil {
		return handleServiceError(c, err)
	}

	return c.JSON(http.StatusOK, status)
}

// ClearReferenceImage handles DELETE /face/clear-reference/:sessionId
func (h *Handler) ClearReferenceImage(c echo.Context) error {
	sessionID := c.Param("sessionId")

	if strings.TrimSpace(sessionID) == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "session_id is required",
		})
	}

	if err := h.service.ClearReferenceImage(sessionID); err != nil {
		return handleServiceError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Reference image cleared successfully",
	})
}

// validateRegisterRequest validates the register base face request
func validateRegisterRequest(req *RegisterBaseFaceRequest) error {
	if strings.TrimSpace(req.SessionID) == "" {
		return errors.New("session_id is required")
	}
	return nil
}

// validateCompareFolderRequest validates the compare folder request
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

// validateImageFile validates the uploaded image file
func validateImageFile(file *multipart.FileHeader) error {
	const maxFileSize = 10 * 1024 * 1024 // 10MB

	if file.Size > maxFileSize {
		return errors.New("image file size exceeds maximum allowed size of 10MB")
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

// handleServiceError maps service errors to appropriate HTTP responses
func handleServiceError(c echo.Context, err error) error {
	errMsg := err.Error()

	if strings.Contains(errMsg, "Session not found") ||
		strings.Contains(errMsg, "No base face registered") {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "No base face registered for this session",
		})
	}

	if strings.Contains(errMsg, "No face detected") {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "No face detected in image",
		})
	}

	if strings.Contains(errMsg, "Multiple faces detected") {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Multiple faces detected, please use image with single face",
		})
	}

	if strings.Contains(errMsg, "Invalid image format") {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid image format",
		})
	}

	if strings.Contains(errMsg, "service unavailable") ||
		strings.Contains(errMsg, "face comparison service unavailable") {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "Face comparison service is temporarily unavailable. Please try again later.",
		})
	}

	if strings.Contains(errMsg, "timeout") ||
		strings.Contains(errMsg, "deadline exceeded") {
		return c.JSON(http.StatusGatewayTimeout, map[string]string{
			"error": "Request timed out. Please try again with fewer images or a smaller folder.",
		})
	}

	if strings.Contains(errMsg, "failed to parse folder link") {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid folder link",
		})
	}

	if strings.Contains(errMsg, "failed to list images") {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Unable to access folder. Please check the folder link and permissions.",
		})
	}

	if strings.Contains(errMsg, "Job not found") {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "Job not found",
		})
	}

	return c.JSON(http.StatusInternalServerError, map[string]string{
		"error": "An unexpected error occurred. Please try again.",
	})
}
