package download

import (
	"all-me-backend/pkg/models"
	"io"
)

type StorageService interface {
	GetFileStream(item *models.CloudItem, token *models.Token) (io.ReadCloser, error)
}
