package googledrive

type File struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Size         string `json:"size"`
	WebURL       string `json:"webViewLink"`
	MimeType     string `json:"mimeType"`
	LastModified string `json:"modifiedTime"`
	ThumbnailURL string `json:"thumbnailLink"`
}

type APIResponse struct {
	Files         []File `json:"files"`
	NextPageToken string `json:"nextPageToken,omitempty"`
}
