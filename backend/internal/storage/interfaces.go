package storage

import (
	"all-me-backend/pkg/models"
	"io"
)

// Provider defines the interface for storage operations with cloud providers
type Provider interface {
	ListFolderContents(item *models.CloudItem, token *models.Token, pageSize int, nextPageToken string) ([]*models.CloudItem, string, error)
	GetFileStream(item *models.CloudItem, token *models.Token) (io.ReadCloser, error)
	GetFaceRecognitionOptimizedStream(item *models.CloudItem, token *models.Token) (io.ReadCloser, error)
	ParseShareLink(shareURL string, token *models.Token) (*models.CloudItem, error)
}
