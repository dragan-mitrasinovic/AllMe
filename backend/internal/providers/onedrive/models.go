package onedrive

// DriveItem represents an item in OneDrive (used for API responses)
type DriveItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	File *struct {
		MimeType string `json:"mimeType"`
	} `json:"file,omitempty"`
	Folder *struct {
		ChildCount int `json:"childCount"`
	} `json:"folder,omitempty"`
	ParentReference *struct {
		Path    string `json:"path"`
		DriveId string `json:"driveId"`
		Id      string `json:"id"`
	} `json:"parentReference,omitempty"`
	DownloadURL string         `json:"@microsoft.graph.downloadUrl"`
	Thumbnails  []ThumbnailSet `json:"thumbnails,omitempty"`
}

// ThumbnailSet represents a set of thumbnails for an item
type ThumbnailSet struct {
	ID       string    `json:"id"`
	Small    Thumbnail `json:"small,omitempty"`
	Medium   Thumbnail `json:"medium,omitempty"`
	Large    Thumbnail `json:"large,omitempty"`
	C400x400 Thumbnail `json:"c400x400,omitempty"` // Custom 400px thumbnail
}

// Thumbnail represents a single thumbnail
type Thumbnail struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// APIResponse represents the response structure from OneDrive API
type APIResponse struct {
	Value    []DriveItem `json:"value"`
	NextLink string      `json:"@odata.nextLink,omitempty"`
}

// APIResponseUser represents user information from OneDrive
type APIResponseUser struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	Mail        string `json:"mail"`
}
