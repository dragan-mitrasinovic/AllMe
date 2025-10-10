package face

import "all-me-backend/pkg/models"

// RegisterBaseFaceRequest represents the request for registering a base face
type RegisterBaseFaceRequest struct {
	SessionID string `form:"session_id"`
}

// RegisterBaseFaceResponse represents the response for base face registration
type RegisterBaseFaceResponse struct {
	Success bool `json:"success"`
}

// CompareFolderRequest represents the request for comparing folder images
type CompareFolderRequest struct {
	SessionID  string `json:"session_id"`
	FolderLink string `json:"folder_link"`
	Provider   string `json:"provider"`
	Recursive  bool   `json:"recursive"`
}

// CompareFolderResponse represents the response for folder comparison
type CompareFolderResponse struct {
	JobID  string `json:"job_id"`
	Status string `json:"status"`
}

// JobStatusResponse represents the response for job status
type JobStatusResponse struct {
	JobID        string              `json:"job_id"`
	Status       string              `json:"status"`
	Progress     int                 `json:"progress"`
	CurrentImage int                 `json:"current_image"`
	TotalImages  int                 `json:"total_images"`
	MatchesFound int                 `json:"matches_found"`
	Message      string              `json:"message"`
	Matches      []*models.CloudItem `json:"matches,omitempty"`
	Error        string              `json:"error,omitempty"`
}

// pythonRegisterRequest represents the request payload for Python service registration
type pythonRegisterRequest struct {
	SessionID string `json:"session_id"`
	Image     string `json:"image"`
}

// pythonRegisterResponse represents the response from Python service registration
type pythonRegisterResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// pythonCompareBatchRequest represents the request payload for Python service batch comparison
type pythonCompareBatchRequest struct {
	SessionID string   `json:"session_id"`
	Images    []string `json:"images"`
}

// pythonCompareBatchResponse represents the response from Python service batch comparison
type pythonCompareBatchResponse struct {
	JobID  string `json:"job_id"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// pythonJobStatusResponse represents the job status response from Python service
type pythonJobStatusResponse struct {
	JobID        string              `json:"job_id"`
	Status       string              `json:"status"`
	Progress     int                 `json:"progress"`
	CurrentImage int                 `json:"current_image"`
	TotalImages  int                 `json:"total_images"`
	MatchesFound int                 `json:"matches_found"`
	Message      string              `json:"message"`
	Matches      []pythonMatchResult `json:"matches,omitempty"`
	Error        string              `json:"error,omitempty"`
}

// pythonMatchResult represents a single match result from Python service
type pythonMatchResult struct {
	Index    int     `json:"index"`
	Distance float64 `json:"distance"`
}
