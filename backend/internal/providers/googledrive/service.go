package googledrive

import (
	"all-me-backend/pkg/models"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

// Service provides all Google Drive operations in one place
// Implements both auth.AuthProvider and storage.StorageProvider interfaces
type Service struct {
	httpClient *http.Client
	baseURL    string
	config     *models.OAuthConfig
}

// NewGoogleDriveService creates a new Google Drive service
func NewGoogleDriveService() *Service {
	return &Service{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    "https://www.googleapis.com/drive/v3",
		config: &models.OAuthConfig{
			ClientID:     os.Getenv("GOOGLEDRIVE_CLIENT_ID"),
			ClientSecret: os.Getenv("GOOGLEDRIVE_CLIENT_SECRET"),
			RedirectURI:  os.Getenv("GOOGLEDRIVE_REDIRECT_URI"),
			Scopes:       []string{"https://www.googleapis.com/auth/drive.readonly"},
			AuthURL:      "https://accounts.google.com/o/oauth2/v2/auth",
			TokenURL:     "https://oauth2.googleapis.com/token",
			Provider:     "googledrive",
		},
	}
}

// GetOAuthConfig returns the OAuth configuration for Google Drive
func (s *Service) GetOAuthConfig() *models.OAuthConfig {
	return s.config
}

// BuildAuthURL constructs the OAuth authorization URL for Google Drive
func (s *Service) BuildAuthURL(state string) (string, error) {
	params := url.Values{}
	params.Add("client_id", s.config.ClientID)
	params.Add("redirect_uri", s.config.RedirectURI)
	params.Add("response_type", "code")
	params.Add("scope", strings.Join(s.config.Scopes, " "))
	params.Add("state", state)
	// Google Drive doesn't need additional parameters like OneDrive

	authURL := s.config.AuthURL + "?" + params.Encode()
	return authURL, nil
}

// ListFolderContents lists all items in a Google Drive folder with pagination support
func (s *Service) ListFolderContents(item *models.CloudItem, token *models.Token, pageSize int, nextPageToken string) ([]*models.CloudItem, string, error) {
	// Build the API URL with query parameters
	baseURL := s.baseURL + "/files"
	params := url.Values{}

	// Query for all items in the specified folder (files and folders)
	query := fmt.Sprintf("'%s' in parents", item.ID)
	params.Set("q", query)

	// Request specific fields
	params.Set("fields", "nextPageToken,files(id,name,mimeType,size,webViewLink,thumbnailLink)")

	// Add pagination parameters
	if pageSize > 0 {
		params.Set("pageSize", fmt.Sprintf("%d", pageSize))
	}
	if nextPageToken != "" {
		params.Set("pageToken", nextPageToken)
	}

	apiURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())

	// Create HTTP request
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create request: %w", err)
	}

	// Add authorization header
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token.AccessToken))
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return nil, "", s.handleAPIError(resp)
	}

	// Parse response
	var driveResp APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&driveResp); err != nil {
		return nil, "", fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert Google Drive files to CloudItem format
	var items []*models.CloudItem
	for _, file := range driveResp.Files {
		// Parse size (Google Drive returns size as string)
		var size int64
		if file.Size != "" {
			fmt.Sscanf(file.Size, "%d", &size)
		}

		// Check if this is a folder
		isFolder := file.MimeType == "application/vnd.google-apps.folder"

		// Set URLs for files (not folders)
		var downloadURL, faceRecognitionOptimizedURL, thumbnailURL string
		if !isFolder {
			// Full resolution for downloads
			downloadURL = fmt.Sprintf("%s/files/%s?alt=media", s.baseURL, file.ID)

			// For images, add face recognition optimized and thumbnail URLs
			if strings.HasPrefix(file.MimeType, "image/") {
				// Face Recognition Optimized: 800px optimized size for face recognition processing
				faceRecognitionOptimizedURL = fmt.Sprintf("%s/files/%s?alt=media&sz=s800", s.baseURL, file.ID)
				// Thumbnail: 400px optimized size for frontend display
				thumbnailURL = fmt.Sprintf("%s/files/%s?alt=media&sz=s400", s.baseURL, file.ID)
			}
		}

		cloudItem := &models.CloudItem{
			ID:                          file.ID,
			Name:                        file.Name,
			MimeType:                    file.MimeType,
			IsFolder:                    isFolder,
			Provider:                    "googledrive",
			DownloadURL:                 downloadURL,                 // Full resolution
			FaceRecognitionOptimizedURL: faceRecognitionOptimizedURL, // 800px optimized for face recognition
			ThumbnailURL:                thumbnailURL,                // 400px optimized for display
		}
		items = append(items, cloudItem)
	}

	return items, driveResp.NextPageToken, nil
}

// GetFileStream retrieves a file stream for downloading (full resolution)
func (s *Service) GetFileStream(item *models.CloudItem, token *models.Token) (io.ReadCloser, error) {
	if item.DownloadURL == "" {
		return nil, fmt.Errorf("download URL not available for item %s", item.ID)
	}

	return s.downloadFromURL(item.DownloadURL, token)
}

// GetFaceRecognitionOptimizedStream retrieves an optimized stream (800px) for face recognition processing
func (s *Service) GetFaceRecognitionOptimizedStream(item *models.CloudItem, token *models.Token) (io.ReadCloser, error) {
	if item.FaceRecognitionOptimizedURL == "" {
		// Fall back to full resolution if optimized version not available
		return s.GetFileStream(item, token)
	}

	return s.downloadFromURL(item.FaceRecognitionOptimizedURL, token)
}

// GetThumbnailStream retrieves a thumbnail stream from a Google Drive thumbnail URL
func (s *Service) GetThumbnailStream(thumbnailURL string, token *models.Token) (io.ReadCloser, error) {
	if thumbnailURL == "" {
		return nil, fmt.Errorf("thumbnail URL is empty")
	}

	// Google Drive thumbnail URLs from the API (thumbnailLink) need authentication
	// CDN URLs (lh3.googleusercontent.com) don't need authentication
	needsAuth := strings.Contains(thumbnailURL, "googleapis.com")

	req, err := http.NewRequest("GET", thumbnailURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create thumbnail request: %w", err)
	}

	if needsAuth {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token.AccessToken))
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch thumbnail: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("thumbnail request failed with status: %d", resp.StatusCode)
	}

	return resp.Body, nil
}

// downloadFromURL is a helper to download from any Google Drive URL
func (s *Service) downloadFromURL(url string, token *models.Token) (io.ReadCloser, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create download request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token.AccessToken))

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute download request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, s.handleAPIError(resp)
	}

	return resp.Body, nil
}

// ParseShareLink parses a Google Drive share link to extract folder information and fetch folder details
func (s *Service) ParseShareLink(shareURL string, token *models.Token) (*models.CloudItem, error) {
	// Clean the URL
	cleanURL := strings.TrimSpace(shareURL)
	cleanURL = strings.TrimSuffix(cleanURL, "/")

	if err := s.validateShareLink(cleanURL); err != nil {
		return nil, err
	}

	parsedURL, err := url.Parse(cleanURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL format: %w", err)
	}

	// Extract folder ID from various Google Drive URL formats
	folderID, err := s.extractFolderID(parsedURL)
	if err != nil {
		return nil, err
	}

	// Fetch folder information using the extracted ID
	folderInfo, err := s.getFolderInfo(folderID, token)
	if err != nil {
		return nil, fmt.Errorf("failed to get folder info: %w", err)
	}

	// Set the provider field
	folderInfo.Provider = "googledrive"

	return folderInfo, nil
}

// getFolderInfo retrieves information about a Google Drive folder (internal method)
func (s *Service) getFolderInfo(folderID string, token *models.Token) (*models.CloudItem, error) {
	// Build the API URL
	apiURL := fmt.Sprintf("%s/files/%s?fields=id,name,mimeType", s.baseURL, folderID)

	// Create HTTP request
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authorization header
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token.AccessToken))
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return nil, s.handleAPIError(resp)
	}

	// Parse response
	var file File
	if err := json.NewDecoder(resp.Body).Decode(&file); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Ensure it's a folder
	if file.MimeType != "application/vnd.google-apps.folder" {
		return nil, fmt.Errorf("item %s is not a folder", folderID)
	}

	return &models.CloudItem{
		ID:       file.ID,
		Name:     file.Name,
		MimeType: file.MimeType,
		IsFolder: true,
	}, nil
}

// validateShareLink checks if the URL is a valid Google Drive share link
func (s *Service) validateShareLink(shareURL string) error {
	// Clean the URL
	cleanURL := strings.TrimSpace(shareURL)
	if cleanURL == "" {
		return fmt.Errorf("share URL cannot be empty")
	}

	parsedURL, err := url.Parse(cleanURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	// Check if it has a valid scheme
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("URL must use http or https scheme")
	}

	// Check if it's a Google Drive domain
	host := strings.ToLower(parsedURL.Host)
	validHosts := []string{
		"drive.google.com",
		"docs.google.com",
	}

	isValidHost := false
	for _, validHost := range validHosts {
		if host == validHost || strings.HasSuffix(host, "."+validHost) {
			isValidHost = true
			break
		}
	}

	if !isValidHost {
		return fmt.Errorf("not a Google Drive share link (invalid host: %s)", host)
	}

	// Check if the URL contains folder indicators
	path := parsedURL.Path
	query := parsedURL.Query()
	hasFolderIndicator := strings.Contains(path, "/folders/") ||
		strings.Contains(path, "/drive/") ||
		query.Get("id") != ""

	if !hasFolderIndicator {
		return fmt.Errorf("URL does not appear to be a Google Drive folder link")
	}

	return nil
}

// extractFolderID extracts folder ID from various Google Drive URL formats
func (s *Service) extractFolderID(parsedURL *url.URL) (string, error) {
	path := parsedURL.Path
	query := parsedURL.Query()

	// Format 1: /drive/folders/{folder_id} (most common)
	if strings.Contains(path, "/folders/") {
		re := regexp.MustCompile(`/folders/([a-zA-Z0-9_-]+)`)
		matches := re.FindStringSubmatch(path)
		if len(matches) > 1 && s.looksLikeFolderID(matches[1]) {
			return matches[1], nil
		}
	}

	// Format 2: /drive/u/{user_number}/folders/{folder_id} (multi-account)
	if strings.Contains(path, "/drive/u/") && strings.Contains(path, "/folders/") {
		re := regexp.MustCompile(`/drive/u/\d+/folders/([a-zA-Z0-9_-]+)`)
		matches := re.FindStringSubmatch(path)
		if len(matches) > 1 && s.looksLikeFolderID(matches[1]) {
			return matches[1], nil
		}
	}

	// Format 3: /open?id={folder_id} (legacy format)
	if strings.Contains(path, "/open") {
		if id := query.Get("id"); id != "" && s.looksLikeFolderID(id) {
			return id, nil
		}
	}

	// Format 4: Query parameter 'id' (alternative format)
	if id := query.Get("id"); id != "" && s.looksLikeFolderID(id) {
		return id, nil
	}

	// Format 5: Extract from any path segment that looks like a folder ID
	// This is a fallback for unexpected formats
	pathParts := strings.Split(strings.Trim(path, "/"), "/")
	for _, part := range pathParts {
		if s.looksLikeFolderID(part) {
			return part, nil
		}
	}

	return "", fmt.Errorf("could not extract folder ID from Google Drive link")
}

// looksLikeFolderID checks if a string looks like a Google Drive folder ID
func (s *Service) looksLikeFolderID(str string) bool {
	// Google Drive folder IDs typically:
	// - Are 25-44 characters long
	// - Contain alphanumeric characters, hyphens, and underscores
	// - Follow a specific pattern
	if len(str) < 25 || len(str) > 44 {
		return false
	}

	// Check if it matches the Google Drive ID pattern
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]+$`, str)
	return matched
}

// handleAPIError processes Google Drive API error responses
func (s *Service) handleAPIError(resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	var errorResponse struct {
		Error struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Status  string `json:"status"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &errorResponse); err != nil {
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return fmt.Errorf("google Drive API error (%d): %s - %s",
		resp.StatusCode, errorResponse.Error.Status, errorResponse.Error.Message)
}

// getEnvOrDefault returns environment variable value or default if not set
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
