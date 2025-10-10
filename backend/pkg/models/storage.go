package models

// CloudItem represents a file in cloud storage
type CloudItem struct {
	ID                          string   `json:"id"`
	Name                        string   `json:"name"`
	MimeType                    string   `json:"mime_type"`
	IsFolder                    bool     `json:"is_folder"`
	Provider                    string   `json:"provider"`                                 // "onedrive" or "googledrive"
	DownloadURL                 string   `json:"download_url"`                             // Full resolution (for ZIP downloads)
	FaceRecognitionOptimizedURL string   `json:"face_recognition_optimized_url,omitempty"` // 800px optimized for face recognition
	ThumbnailURL                string   `json:"thumbnail_url,omitempty"`                  // 400px optimized for frontend display
	MatchDistance               *float64 `json:"match_distance,omitempty"`                 // Face recognition match distance (0.0-1.0, lower is better)
	ParentShareToken            string   `json:"-"`                                        // OneDrive share token for accessing subfolders (not sent to frontend)
	ParentPath                  string   `json:"-"`                                        // Path from share root to this item (not sent to frontend)
	DriveID                     string   `json:"-"`                                        // OneDrive drive ID for direct access (not sent to frontend)
}

// DownloadRequest represents a request to download files
type DownloadRequest struct {
	Files  []*CloudItem `json:"files"`
	Token  *Token       `json:"token"`
	Format string       `json:"format"` // "single" or "zip"
}
