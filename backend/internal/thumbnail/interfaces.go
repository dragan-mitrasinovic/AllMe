package thumbnail

import (
	"all-me-backend/pkg/models"
	"io"
)

// Provider defines the interface for thumbnail operations with cloud providers
type Provider interface {
	GetThumbnailStream(thumbnailURL string, token *models.Token) (io.ReadCloser, error)
}
