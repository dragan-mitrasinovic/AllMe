package onedrive

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

type ThumbnailSet struct {
	ID       string    `json:"id"`
	Small    Thumbnail `json:"small,omitempty"`
	Medium   Thumbnail `json:"medium,omitempty"`
	Large    Thumbnail `json:"large,omitempty"`
	C400x400 Thumbnail `json:"c400x400,omitempty"` // Custom 400px thumbnail
}

type Thumbnail struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type APIResponse struct {
	Value    []DriveItem `json:"value"`
	NextLink string      `json:"@odata.nextLink,omitempty"`
}
