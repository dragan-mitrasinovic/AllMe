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
	FilesReadScope      = "files.read"
	AuthCodeGrantType   = "authorization_code"
)

type ServiceConfig struct {
	SharesBaseUrl string
	AuthTokenUrl  string

	ClientId     string
	ClientSecret string
	RedirectUri  string
}

type Service struct {
	client *http.Client
	config ServiceConfig
}

func NewService(client *http.Client, config ServiceConfig) *Service {
	return &Service{
		client: client,
		config: config,
	}
}

func (s *Service) GetImagesFromSharedFolder(folderLink, authCode string) ([]DriveImage, error) {
	authToken, err := s.getUserDelegatedToken(authCode)
	if err != nil {
		return nil, err
	}

	driveItems, err := s.getItemsFromOneDrive(folderLink, authToken)
	if err != nil {
		return nil, err
	}

	var images []DriveImage
	for _, item := range driveItems {
		if isImage(item) {
			images = append(images, DriveImage{
				Name:   item.Name,
				Source: item.Source,
			})
		}
	}

	return images, nil
}

func (s *Service) getUserDelegatedToken(authCode string) (string, error) {
	requestUrl := s.config.AuthTokenUrl
	req, err := http.NewRequest(http.MethodPost, requestUrl, nil)
	if err != nil {
		return "", errors.New("failed to create request")
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	query := req.URL.Query()
	query.Add("client_id", s.config.ClientId)
	query.Add("client_secret", s.config.ClientSecret)
	query.Add("scope", FilesReadScope)
	query.Add("grant_type", AuthCodeGrantType)
	query.Add("code", authCode)
	query.Add("redirect_uri", s.config.RedirectUri)
	req.URL.RawQuery = query.Encode()

	resp, err := s.client.Do(req)
	if err != nil {
		return "", errors.New("failed to send request")
	}

	if resp.StatusCode != http.StatusOK {
		return "", errors.New("received non-200 response code")
	}

	var response AuthTokenResponse
	if err = json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", errors.New("failed to decode response")
	}

	err = resp.Body.Close()
	if err != nil {
		return "", errors.New("failed to close response body")
	}

	return response.AuthToken, nil
}

func (s *Service) getItemsFromOneDrive(folderLink, authToken string) ([]DriveItem, error) {
	encodedFolderLink := encodeFolderLink(folderLink)
	requestUrl := fmt.Sprintf("%s/%s/driveItem", s.config.SharesBaseUrl, encodedFolderLink)

	req, err := http.NewRequest(http.MethodGet, requestUrl, nil)
	if err != nil {
		return nil, errors.New("failed to create request")
	}

	req.Header.Set("Authorization", "Bearer "+authToken)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, errors.New("failed to send request")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("received non-200 response code")
	}

	var response FolderContentsResponse
	if err = json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, errors.New("failed to decode response")
	}

	err = resp.Body.Close()
	if err != nil {
		return nil, errors.New("failed to close response body")
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

func isImage(item DriveItem) bool {
	return strings.HasPrefix(item.File.MimeType, ImageMimeTypePrefix)
}
