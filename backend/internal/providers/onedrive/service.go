package onedrive

import (
	"all-me-backend/pkg/models"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Service provides all OneDrive operations in one place
type Service struct {
	httpClient *http.Client
	baseURL    string
	config     *models.OAuthConfig
}

// NewOneDriveService creates a new OneDrive service
func NewOneDriveService() *Service {
	return &Service{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    "https://graph.microsoft.com/v1.0",
		config: &models.OAuthConfig{
			ClientID:     os.Getenv("ONEDRIVE_CLIENT_ID"),
			ClientSecret: os.Getenv("ONEDRIVE_CLIENT_SECRET"),
			RedirectURI:  os.Getenv("ONEDRIVE_REDIRECT_URI"),
			Scopes:       []string{"Files.Read.All"},
			AuthURL:      "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
			TokenURL:     "https://login.microsoftonline.com/common/oauth2/v2.0/token",
			Provider:     "onedrive",
		},
	}
}

// GetOAuthConfig returns the OAuth configuration for OneDrive
func (s *Service) GetOAuthConfig() *models.OAuthConfig {
	return s.config
}

// BuildAuthURL constructs the OAuth authorization URL for OneDrive
func (s *Service) BuildAuthURL(state string) (string, error) {
	params := url.Values{}
	params.Add("client_id", s.config.ClientID)
	params.Add("redirect_uri", s.config.RedirectURI)
	params.Add("response_type", "code")
	params.Add("scope", strings.Join(s.config.Scopes, " "))
	params.Add("state", state)
	params.Add("response_mode", "query") // OneDrive-specific parameter

	authURL := s.config.AuthURL + "?" + params.Encode()
	return authURL, nil
}

// ListFolderContents lists all items in a OneDrive folder with pagination support
func (s *Service) ListFolderContents(item *models.CloudItem, token *models.Token, pageSize int, nextPageToken string) ([]*models.CloudItem, string, error) {
	// Build the API URL
	var apiURL string
	var shareToken string
	var currentPath string
	var driveID string

	if nextPageToken != "" {
		// Use the next page URL directly
		apiURL = nextPageToken
		// Preserve share token, path, and driveID from the parent item
		shareToken = item.ParentShareToken
		currentPath = item.ParentPath
		driveID = item.DriveID
	} else {
		// Determine if this is a share token or has a parent share token
		// Share tokens start with "u!" or "s!" (encoded share links)
		isRootShare := strings.HasPrefix(item.ID, "u!") || strings.HasPrefix(item.ID, "s!")

		// Add pagination and thumbnail parameters
		params := url.Values{}
		if pageSize > 0 {
			params.Add("$top", fmt.Sprintf("%d", pageSize))
		}
		// Request custom thumbnail sizes: c400x400 for display, large (800px) for face recognition
		// Format: $expand=thumbnails($select=c400x400,large)
		params.Add("$expand", "thumbnails($select=c400x400,large)")

		if isRootShare {
			// This is the root shared folder - use shares API directly
			shareToken = item.ID
			currentPath = ""
			driveID = item.DriveID
			apiURL = fmt.Sprintf("%s/shares/%s/driveItem/children", s.baseURL, shareToken)
		} else if item.DriveID != "" {
			// This is a subfolder within a share - use drives API with actual drive and item IDs
			// The shares API doesn't support subfolder navigation, but we can use the drives API
			// with the driveId from parentReference to access subfolders reliably
			shareToken = item.ParentShareToken
			currentPath = item.ParentPath
			driveID = item.DriveID

			apiURL = fmt.Sprintf("%s/drives/%s/items/%s/children", s.baseURL, item.DriveID, item.ID)
		} else {
			// This is a regular folder in user's own drive (not a share)
			apiURL = fmt.Sprintf("%s/me/drive/items/%s/children", s.baseURL, item.ID)
			shareToken = ""
			currentPath = ""
			driveID = ""
		}

		if len(params) > 0 {
			apiURL += "?" + params.Encode()
		}
	}

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

	// Read response body for logging and parsing
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read response body: %w", err)
	}

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("OneDrive list API error (status %d) for folder ID '%s' at URL '%s': %s",
			resp.StatusCode, item.ID, apiURL, string(body))
	}

	// Parse response as standard API response (both initial and paginated requests use same format)
	var oneDriveResp APIResponse
	if err := json.Unmarshal(body, &oneDriveResp); err != nil {
		return nil, "", fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert OneDrive items to CloudItem format
	var items []*models.CloudItem
	for _, driveItem := range oneDriveResp.Value {
		cloudItem := s.convertDriveItemToCloudItem(driveItem, shareToken, currentPath, driveID)
		items = append(items, cloudItem)
	}

	return items, oneDriveResp.NextLink, nil
}

// convertDriveItemToCloudItem converts a OneDrive DriveItem to CloudItem format
func (s *Service) convertDriveItemToCloudItem(item DriveItem, shareToken string, parentPath string, parentDriveID string) *models.CloudItem {
	// Check if it's a folder or file
	isFolder := item.Folder != nil

	var mimeType string
	if item.File != nil {
		mimeType = item.File.MimeType
	} else if isFolder {
		mimeType = "application/vnd.onedrive.folder"
	}

	// Build the path for this item
	var itemPath string
	if parentPath == "" {
		// This is a direct child of the root share
		itemPath = item.Name
	} else {
		// This is nested deeper
		itemPath = parentPath + "/" + item.Name
	}

	// Get drive ID from item or parent
	var driveID string
	if item.ParentReference != nil && item.ParentReference.DriveId != "" {
		driveID = item.ParentReference.DriveId
	} else {
		driveID = parentDriveID
	}

	// Set URLs for files
	downloadURL := item.DownloadURL // Full resolution from OneDrive API
	var faceRecognitionOptimizedURL, thumbnailURL string

	if !isFolder && item.File != nil && strings.HasPrefix(mimeType, "image/") {
		// Use thumbnail URLs from the API response if available
		if len(item.Thumbnails) > 0 {
			thumbnailSet := item.Thumbnails[0]

			// Use large thumbnail (800px) for face recognition processing
			if thumbnailSet.Large.URL != "" {
				faceRecognitionOptimizedURL = thumbnailSet.Large.URL
			}

			// Use custom 400px thumbnail for display (higher quality than medium's 176px)
			if thumbnailSet.C400x400.URL != "" {
				thumbnailURL = thumbnailSet.C400x400.URL
			}
		}
	}

	return &models.CloudItem{
		ID:                          item.ID,
		Name:                        item.Name,
		MimeType:                    mimeType,
		IsFolder:                    isFolder,
		Provider:                    "onedrive",
		DownloadURL:                 downloadURL,                 // Full resolution
		FaceRecognitionOptimizedURL: faceRecognitionOptimizedURL, // 800px optimized for face recognition
		ThumbnailURL:                thumbnailURL,                // 400px optimized for display
		ParentShareToken:            shareToken,                  // Preserve share token for recursive access
		ParentPath:                  itemPath,                    // Path from share root for API navigation
		DriveID:                     driveID,                     // OneDrive drive ID for direct access
	}
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

	// For OneDrive share thumbnails, they should be handled by the thumbnail proxy
	// This method is mainly for direct download URLs
	return s.downloadFromURL(item.FaceRecognitionOptimizedURL, token)
}

// GetThumbnailStream retrieves a thumbnail stream from a OneDrive thumbnail URL
func (s *Service) GetThumbnailStream(thumbnailURL string, token *models.Token) (io.ReadCloser, error) {
	if thumbnailURL == "" {
		return nil, fmt.Errorf("thumbnail URL is empty")
	}

	// All OneDrive thumbnail URLs are handled the same way
	return s.downloadFromURL(thumbnailURL, token)
}

// downloadFromURL is a helper to download from any OneDrive URL
func (s *Service) downloadFromURL(url string, token *models.Token) (io.ReadCloser, error) {
	downloadReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create download request: %w", err)
	}

	// Add authorization header for API URLs (shares API and thumbnails require auth)
	// Regular download URLs from @microsoft.graph.downloadUrl don't need auth
	if strings.Contains(url, "/thumbnails/") || strings.Contains(url, "/shares/") {
		downloadReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token.AccessToken))
	}

	downloadResp, err := s.httpClient.Do(downloadReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute download request: %w", err)
	}

	if downloadResp.StatusCode != http.StatusOK {
		downloadResp.Body.Close()
		return nil, fmt.Errorf("OneDrive download error (status %d)", downloadResp.StatusCode)
	}

	return downloadResp.Body, nil
}

// ParseShareLink parses a OneDrive share link to extract folder information and fetch folder details
func (s *Service) ParseShareLink(shareURL string, token *models.Token) (*models.CloudItem, error) {
	if err := s.validateShareLink(shareURL); err != nil {
		return nil, err
	}

	// Encode the share URL to get the share token
	shareToken := s.encodeShareToken(shareURL)

	// Use the shares API directly with the original URL
	// This avoids the need to reconstruct URLs or hardcode user IDs
	folderInfo, err := s.getFolderInfoFromShareURL(shareURL, token)
	if err != nil {
		return nil, fmt.Errorf("failed to get folder info: %w", err)
	}

	// Set the provider field and use share token as ID for ListFolderContents
	folderInfo.Provider = "onedrive"
	folderInfo.ID = shareToken // Use share token instead of actual folder ID

	return folderInfo, nil
}

// getFolderInfoFromShareURL retrieves information about a OneDrive folder using the shares API
func (s *Service) getFolderInfoFromShareURL(shareURL string, token *models.Token) (*models.CloudItem, error) {
	// Encode the share URL for the shares API
	shareToken := s.encodeShareToken(shareURL)

	// Build the API URL for shares
	apiURL := fmt.Sprintf("%s/shares/%s/driveItem", s.baseURL, shareToken)

	// Create HTTP request
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create shares request: %w", err)
	}

	// Add authorization header
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token.AccessToken))
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute shares request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body for logging and parsing
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("shares API failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var item DriveItem
	if err := json.Unmarshal(body, &item); err != nil {
		return nil, fmt.Errorf("failed to decode shares response: %w", err)
	}

	// Extract drive ID from the response if available
	var driveID string
	if item.ParentReference != nil && item.ParentReference.DriveId != "" {
		driveID = item.ParentReference.DriveId
	}

	return &models.CloudItem{
		ID:       shareToken, // Use share token as ID for consistent access
		Name:     item.Name,
		MimeType: "application/vnd.onedrive.folder",
		IsFolder: true,
		DriveID:  driveID, // Store drive ID for subfolder navigation
	}, nil
}

// validateShareLink checks if the URL is a valid OneDrive share link
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

	// Check if it's a OneDrive domain
	host := strings.ToLower(parsedURL.Host)
	validHosts := []string{
		"1drv.ms",
		"onedrive.live.com",
		"d.docs.live.net",
		"onedrive.com",
	}

	isValidHost := false
	for _, validHost := range validHosts {
		if host == validHost || strings.HasSuffix(host, "."+validHost) {
			isValidHost = true
			break
		}
	}

	if !isValidHost {
		return fmt.Errorf("not a OneDrive share link (invalid host: %s)", host)
	}

	// Additional validation for 1drv.ms short links
	if host == "1drv.ms" {
		path := strings.Trim(parsedURL.Path, "/")
		if path == "" {
			return fmt.Errorf("OneDrive short link is missing path")
		}
		// Should start with /f/ for folders
		if !strings.HasPrefix(path, "f/") {
			return fmt.Errorf("OneDrive link does not appear to be a folder link")
		}
	}

	return nil
}

// encodeShareToken encodes a share URL for use with OneDrive shares API
func (s *Service) encodeShareToken(shareURL string) string {
	// Clean the URL first (remove trailing slashes, normalize)
	cleanURL := strings.TrimSpace(shareURL)
	cleanURL = strings.TrimSuffix(cleanURL, "/")

	// Step 1: Base64 encode the URL
	encoded := base64.StdEncoding.EncodeToString([]byte(cleanURL))

	// Step 2: Convert to unpadded base64url format
	// Remove = characters from the end
	encoded = strings.TrimRight(encoded, "=")
	// Replace / with _ and + with -
	encoded = strings.ReplaceAll(encoded, "/", "_")
	encoded = strings.ReplaceAll(encoded, "+", "-")

	// Step 3: Append u! to the beginning
	shareToken := "u!" + encoded

	return shareToken
}
