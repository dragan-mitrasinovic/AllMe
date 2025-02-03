package onedrive

type DriveImage struct {
	Name   string `json:"name"`
	Source string `json:"@microsoft.graph.downloadUrl"`
}

type File struct {
	MimeType string `json:"mimeType"`
}

type DriveItem struct {
	Name   string `json:"name"`
	File   File   `json:"file"`
	Source string `json:"@microsoft.graph.downloadUrl"`
}

type FolderContentsResponse struct {
	Value []DriveItem `json:"value"`
}

type AuthTokenResponse struct {
	AuthToken string `json:"access_token"`
}
