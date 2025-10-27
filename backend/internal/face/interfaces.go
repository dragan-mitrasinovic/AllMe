package face

import (
	"all-me-backend/pkg/models"
	"io"
)

type StorageService interface {
	ParseShareLink(shareURL string, token *models.Token) (*models.CloudItem, error)
	ListImages(item *models.CloudItem, token *models.Token, recursive bool) ([]*models.CloudItem, error)
	GetFaceRecognitionOptimizedStream(item *models.CloudItem, token *models.Token) (io.ReadCloser, error)
}
