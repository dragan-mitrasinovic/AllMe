package onedrive

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

const (
	ImageMimeTypePrefix = "image/"
)

type Service struct {
	client  *http.Client
	baseUrl string
}

func NewService(client *http.Client, baseUrl string) *Service {
	return &Service{
		client:  client,
		baseUrl: baseUrl,
	}
}

func (s *Service) GetImagesFromSharedFolder(folderLink, authToken string) ([]DriveImage, error) {
	encodedLink := encodeFolderLink(folderLink)

	driveItems, err := s.getItemsFromOneDrive(encodedLink, authToken)
	if err != nil {
		return nil, err
	}

	var images []DriveImage
	for _, item := range driveItems {
		if strings.HasPrefix(item.File.MimeType, ImageMimeTypePrefix) {
			images = append(images, DriveImage{
				Name:   item.Name,
				Source: item.Source,
			})
		}
	}

	return images, nil
}

func (s *Service) getItemsFromOneDrive(encodeFolderLink, authToken string) ([]DriveItem, error) {
	requestUrl := fmt.Sprintf("%s/%s/driveItem", s.baseUrl, encodeFolderLink)
	req, err := http.NewRequest(http.MethodGet, requestUrl, nil)
	if err != nil {
		return nil, errors.New("failed to create request")
	}

	req.Header.Set("Authorization", "Bearer "+authToken)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, errors.New("failed to send request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("received non-200 response code")
	}

	var response FolderContentsResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, errors.New("failed to decode response")
	}

	return response.Value, nil
}

func encodeFolderLink(folderLink string) string {
	encoded := base64.StdEncoding.EncodeToString([]byte(folderLink))

	encoded = strings.TrimRight(encoded, "=")
	encoded = strings.ReplaceAll(encoded, "/", "_")
	encoded = strings.ReplaceAll(encoded, "+", "-")

	return "u!" + encoded
}
