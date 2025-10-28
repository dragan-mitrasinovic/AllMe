package storage

import "all-me-backend/pkg/models"

type GetFolderContentsResponse struct {
	Folder   *models.CloudItem   `json:"folder"`
	Contents []*models.CloudItem `json:"contents"`
}
