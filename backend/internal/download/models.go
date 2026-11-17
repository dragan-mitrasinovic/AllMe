package download

import "all-me-backend/pkg/models"

// ZipRequest represents the request body for ZIP download
type ZipRequest struct {
	Files    []*models.CloudItem `json:"files"`
	Provider string              `json:"provider"`
}
