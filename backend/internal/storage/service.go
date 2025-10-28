package storage

import (
	"all-me-backend/pkg/models"
	"fmt"
	"io"
	"net/url"
	"slices"
	"strings"
)

type Service struct {
	googleDriveStorage Provider
	oneDriveStorage    Provider
}

func NewService(
	googleDriveStorage Provider,
	oneDriveStorage Provider,
) *Service {
	return &Service{
		googleDriveStorage: googleDriveStorage,
		oneDriveStorage:    oneDriveStorage,
	}
}

// ParseShareLink extracts folder ID and provider from a cloud storage share link
func (s *Service) ParseShareLink(shareURL string, token *models.Token) (*models.CloudItem, error) {
	cleanURL := strings.TrimSpace(shareURL)
	parsedURL, err := url.Parse(cleanURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL format: %w", err)
	}

	if parsedURL.Scheme == "" {
		return nil, fmt.Errorf("URL must include protocol (http:// or https://)")
	}

	// Route to appropriate provider based on token provider
	switch token.Provider {
	case "onedrive":
		return s.oneDriveStorage.ParseShareLink(cleanURL, token)
	case "googledrive":
		return s.googleDriveStorage.ParseShareLink(cleanURL, token)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", token.Provider)
	}
}

// ListFolderContents lists all items (files and folders) in the specified folder
func (s *Service) ListFolderContents(item *models.CloudItem, token *models.Token) ([]*models.CloudItem, error) {
	switch token.Provider {
	case "onedrive":
		return s.listAllItemsWithPagination(item, token, s.oneDriveStorage)
	case "googledrive":
		return s.listAllItemsWithPagination(item, token, s.googleDriveStorage)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", token.Provider)
	}
}

// ListImages lists all image files in the specified folder
func (s *Service) ListImages(item *models.CloudItem, token *models.Token, recursive bool) ([]*models.CloudItem, error) {
	allItems, err := s.ListFolderContents(item, token)
	if err != nil {
		return nil, err
	}

	images := make([]*models.CloudItem, 0)
	for _, currentItem := range allItems {
		if currentItem.IsFolder && recursive {
			// Recursively get images from subfolder
			subImages, err := s.ListImages(currentItem, token, recursive)
			if err != nil {
				continue
			}

			images = append(images, subImages...)
		} else if !currentItem.IsFolder && IsImageMimeType(currentItem.MimeType) {
			images = append(images, currentItem)
		}
	}

	return images, nil
}

// GetFileStream retrieves a file stream for downloading (full resolution)
func (s *Service) GetFileStream(item *models.CloudItem, token *models.Token) (io.ReadCloser, error) {
	switch token.Provider {
	case "onedrive":
		return s.oneDriveStorage.GetFileStream(item, token)
	case "googledrive":
		return s.googleDriveStorage.GetFileStream(item, token)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", token.Provider)
	}
}

// GetFaceRecognitionOptimizedStream retrieves a 800px image stream optimized for face recognition processing
func (s *Service) GetFaceRecognitionOptimizedStream(item *models.CloudItem, token *models.Token) (io.ReadCloser, error) {
	switch token.Provider {
	case "onedrive":
		return s.oneDriveStorage.GetFaceRecognitionOptimizedStream(item, token)
	case "googledrive":
		return s.googleDriveStorage.GetFaceRecognitionOptimizedStream(item, token)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", token.Provider)
	}
}

// listAllItemsWithPagination handles pagination for listing all items from cloud storage
func (s *Service) listAllItemsWithPagination(item *models.CloudItem, token *models.Token, provider Provider) ([]*models.CloudItem, error) {
	const pageSize = 100
	var allItems []*models.CloudItem
	var nextPageToken string

	for {
		// Get current page of items (files and folders)
		items, nextToken, err := provider.ListFolderContents(item, token, pageSize, nextPageToken)
		if err != nil {
			return nil, fmt.Errorf("failed to list folder contents: %w", err)
		}

		allItems = append(allItems, items...)

		// Check if there are more pages
		if nextToken == "" {
			break
		}
		nextPageToken = nextToken
	}

	// Sort items: folders first, then images, then other files
	s.sortCloudItems(allItems)

	return allItems, nil
}

// sortCloudItems sorts items by type: folders first, then images, then other files
// Within each category, items are sorted alphabetically by name
func (s *Service) sortCloudItems(items []*models.CloudItem) {
	slices.SortFunc(items, func(a, b *models.CloudItem) int {
		if a.IsFolder && !b.IsFolder {
			return -1
		}
		if !a.IsFolder && b.IsFolder {
			return 1
		}

		// If both are not folders, images come before other files
		if !a.IsFolder && !b.IsFolder {
			aIsImage := IsImageMimeType(a.MimeType)
			bIsImage := IsImageMimeType(b.MimeType)

			if aIsImage && !bIsImage {
				return -1
			}
			if !aIsImage && bIsImage {
				return 1
			}
		}

		// Within the same category, sort alphabetically by name (case-insensitive)
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
}

func IsImageMimeType(mimeType string) bool {
	imageMimeTypes := []string{
		"image/jpeg",
		"image/jpg",
		"image/png",
		"image/gif",
		"image/webp",
		"image/bmp",
		"image/svg+xml",
	}
	return slices.Contains(imageMimeTypes, mimeType)
}
