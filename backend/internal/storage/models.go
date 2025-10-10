package storage

import "all-me-backend/pkg/models"

// GetFolderContentsResponse represents the response for getting folder contents
type GetFolderContentsResponse struct {
	Folder   *models.CloudItem   `json:"folder"`
	Contents []*models.CloudItem `json:"contents"`
}
