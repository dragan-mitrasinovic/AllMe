package thumbnail

import (
	"all-me-backend/pkg/models"
	"io"
)

type Provider interface {
	GetThumbnailStream(thumbnailURL string, token *models.Token) (io.ReadCloser, error)
}
