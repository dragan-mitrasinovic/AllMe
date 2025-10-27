package face

import (
	"errors"
	"net/http"
)

var (
	ErrNoBaseFace         = errors.New("no base face registered for this session")
	ErrSessionNotFound    = errors.New("session not found")
	ErrNoFaceDetected     = errors.New("no face detected in image")
	ErrMultipleFaces      = errors.New("multiple faces detected, please use image with single face")
	ErrInvalidImageFormat = errors.New("invalid image format")
	ErrServiceUnavailable = errors.New("face comparison service is temporarily unavailable")
	ErrTimeout            = errors.New("request timed out")
	ErrInvalidFolderLink  = errors.New("invalid folder link")
	ErrFolderAccess       = errors.New("unable to access folder")
	ErrJobNotFound        = errors.New("job not found")
)

type ErrorResponse struct {
	StatusCode int
	Message    string
}

// GetErrorResponse returns appropriate HTTP response for an error
func GetErrorResponse(err error) ErrorResponse {
	switch {
	case errors.Is(err, ErrNoBaseFace):
		return ErrorResponse{http.StatusBadRequest, err.Error()}
	case errors.Is(err, ErrSessionNotFound):
		return ErrorResponse{http.StatusBadRequest, err.Error()}
	case errors.Is(err, ErrNoFaceDetected):
		return ErrorResponse{http.StatusBadRequest, err.Error()}
	case errors.Is(err, ErrMultipleFaces):
		return ErrorResponse{http.StatusBadRequest, err.Error()}
	case errors.Is(err, ErrInvalidImageFormat):
		return ErrorResponse{http.StatusBadRequest, err.Error()}
	case errors.Is(err, ErrServiceUnavailable):
		return ErrorResponse{http.StatusServiceUnavailable, "Face comparison service is temporarily unavailable. Please try again later."}
	case errors.Is(err, ErrTimeout):
		return ErrorResponse{http.StatusGatewayTimeout, "Request timed out. Please try again with fewer images or a smaller folder."}
	case errors.Is(err, ErrInvalidFolderLink):
		return ErrorResponse{http.StatusBadRequest, err.Error()}
	case errors.Is(err, ErrFolderAccess):
		return ErrorResponse{http.StatusBadRequest, "Unable to access folder. Please check the folder link and permissions."}
	case errors.Is(err, ErrJobNotFound):
		return ErrorResponse{http.StatusNotFound, err.Error()}
	default:
		return ErrorResponse{http.StatusInternalServerError, "An unexpected error occurred. Please try again."}
	}
}
