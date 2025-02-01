package onedrive

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

const (
	GraphSharesApiUrl = "https://graph.microsoft.com/v1.0/shares"
)

type Photo struct {
	name    string
	rawData []byte
}

type Service struct {
	client *http.Client
}

func NewService(client *http.Client) *Service {
	return &Service{client: client}
}

func (s *Service) GetPhotosInSharedFolder(folderLink, authToken string) ([]Photo, error) {
	encodedLink := encodeFolderLink(folderLink)

	requestUrl := fmt.Sprintf("%s/%s/driveItem", GraphSharesApiUrl, encodedLink)
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

	return []Photo{
		{name: "example", rawData: nil},
	}, nil
}

func encodeFolderLink(folderLink string) string {
	encoded := base64.StdEncoding.EncodeToString([]byte(folderLink))

	encoded = strings.TrimRight(encoded, "=")
	encoded = strings.ReplaceAll(encoded, "/", "_")
	encoded = strings.ReplaceAll(encoded, "+", "-")

	return "u!" + encoded
}
