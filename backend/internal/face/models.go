package face

import "all-me-backend/pkg/models"

type RegisterBaseFaceRequest struct {
	SessionID string `form:"session_id"`
}

type RegisterBaseFaceResponse struct {
	Success bool `json:"success"`
}

type CompareFolderRequest struct {
	SessionID  string `json:"session_id"`
	FolderLink string `json:"folder_link"`
	Provider   string `json:"provider"`
	Recursive  bool   `json:"recursive"`
}

type CompareFolderResponse struct {
	JobID  string `json:"job_id"`
	Status string `json:"status"`
}

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

type pythonRegisterRequest struct {
	SessionID string `json:"session_id"`
	Image     string `json:"image"`
}

type pythonRegisterResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

type pythonCompareBatchRequest struct {
	SessionID string   `json:"session_id"`
	Images    []string `json:"images"`
}

type pythonCompareBatchResponse struct {
	JobID  string `json:"job_id"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

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

type pythonMatchResult struct {
	Index    int     `json:"index"`
	Distance float64 `json:"distance"`
}
