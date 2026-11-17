package face

import (
	"all-me-backend/internal/storage"
	"errors"
	"fmt"
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
	ErrTooManyImages      = errors.New("too many images in folder")
	ErrNoImages           = errors.New("no images found in folder")
)

// msgUploadPhotoAgain is shown when the face-service doesn't have the user's reference photo (any more).
const msgUploadPhotoAgain = "Your photo of yourself is no longer available. Please upload it again, then start the search."

type ErrorResponse struct {
	StatusCode int
	Message    string
}

// GetErrorResponse returns the HTTP status and a fixed, user-facing message for an error. The messages never
// include the error's own text, which can carry provider or face-service details.
func GetErrorResponse(err error) ErrorResponse {
	switch {
	case errors.Is(err, ErrNoBaseFace), errors.Is(err, ErrSessionNotFound):
		return ErrorResponse{http.StatusBadRequest, msgUploadPhotoAgain}
	case errors.Is(err, ErrNoFaceDetected):
		return ErrorResponse{http.StatusBadRequest, "No face found in that photo. Please pick one where your face is clearly visible."}
	case errors.Is(err, ErrMultipleFaces):
		return ErrorResponse{http.StatusBadRequest, "That photo has more than one face. Please pick one with just you in it."}
	case errors.Is(err, ErrInvalidImageFormat):
		return ErrorResponse{http.StatusBadRequest, "That image couldn't be read. Please use a JPEG or PNG photo."}
	case errors.Is(err, ErrServiceUnavailable):
		return ErrorResponse{http.StatusServiceUnavailable, "Face comparison service is temporarily unavailable. Please try again later."}
	case errors.Is(err, ErrTimeout):
		return ErrorResponse{http.StatusGatewayTimeout, "Request timed out. Please try again with fewer images or a smaller folder."}
	case errors.Is(err, ErrInvalidFolderLink):
		return ErrorResponse{http.StatusBadRequest, "Couldn't open that folder link. Check that it's a shared folder link and that you can open it."}
	case errors.Is(err, ErrNoImages):
		return ErrorResponse{http.StatusBadRequest, "No photos found in this folder."}
	case errors.Is(err, ErrFolderAccess):
		return ErrorResponse{http.StatusBadRequest, "Unable to access folder. Please check the folder link and permissions."}
	case errors.Is(err, ErrTooManyImages):
		return ErrorResponse{http.StatusBadRequest, fmt.Sprintf(
			"This folder has more than %d photos. Please pick a smaller folder or one of its subfolders.",
			storage.MaxImagesPerSearch)}
	case errors.Is(err, ErrJobNotFound):
		return ErrorResponse{http.StatusNotFound, "This search has expired. Please start it again."}
	default:
		return ErrorResponse{http.StatusInternalServerError, "An unexpected error occurred. Please try again."}
	}
}
