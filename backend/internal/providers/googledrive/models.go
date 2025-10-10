package googledrive

// File represents a file in Google Drive
type File struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Size         string `json:"size"`
	WebURL       string `json:"webViewLink"`
	MimeType     string `json:"mimeType"`
	LastModified string `json:"modifiedTime"`
	ThumbnailURL string `json:"thumbnailLink"`
}

// APIResponse represents the response structure from Google Drive API
type APIResponse struct {
	Files         []File `json:"files"`
	NextPageToken string `json:"nextPageToken,omitempty"`
}

// APIResponseUser represents user information from Google Drive
type APIResponseUser struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	Email       string `json:"emailAddress"`
}
